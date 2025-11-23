package handler

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/service"
)

type Storage interface {
	UpdateMetric(model.Metrics) error
	GetMetric(mType, mName string) (model.Metrics, error)
	GetAll() map[string]model.Metrics
}

func MetricRouter(s Storage, cfg config.ServerConfig) chi.Router {
	r := chi.NewRouter()
	r.Use(
		middleware.StripSlashes,
		withLogging,
		withGzip,
	)
	textPlainContentType := middleware.AllowContentType("text/plain")
	r.With(textPlainContentType).Post("/update/{mType}/{mName}/{mValue}", updateMetricHandler(s, cfg.FileStoragePath))
	r.With(textPlainContentType).Get("/value/{mType}/{mName}", getMetricHandler(s))
	r.With(textPlainContentType).Get("/", getMetricListHandler(s))

	applicationJsonContentType := middleware.AllowContentType("application/json")
	r.With(applicationJsonContentType).Post("/update", updateHandler(s, cfg.FileStoragePath))
	r.With(applicationJsonContentType).Post("/value", getHandler(s))
	return r
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		URI := r.RequestURI
		method := r.Method
		start := time.Now()

		responseData := &responseData{
			status: http.StatusOK,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}
		next.ServeHTTP(&lw, r)

		duration := time.Since(start)
		log.Info().
			Str("uri", URI).
			Str("method", method).
			Str("duration", duration.String()).
			Str("status", strconv.Itoa(responseData.status)).
			Str("size", strconv.Itoa(responseData.size)).
			Msg("Request processed")
	})
}

func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportGzip := strings.Contains(acceptEncoding, "gzip")
		if supportGzip {
			cw := newCompressWriter(w)
			defer cw.Close()
			cw.Header().Set("Content-Encoding", "gzip")
			ow = cw
		}

		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				if errors.Is(err, gzip.ErrChecksum) || errors.Is(err, gzip.ErrHeader) {
					ow.WriteHeader(http.StatusBadRequest)
				} else {
					ow.WriteHeader(http.StatusInternalServerError)
					logError(err, "Request error")
				}
				return
			}
			defer cr.Close()
			r.Body = cr
		}

		next.ServeHTTP(ow, r)
	})
}

func updateMetricHandler(s Storage, fileStoragePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mType := chi.URLParam(r, "mType")
		mName := chi.URLParam(r, "mName")
		mValue := chi.URLParam(r, "mValue")

		m, err := buildMetric(mType, mName, mValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = s.UpdateMetric(m)
		if err != nil {
			if errors.Is(err, service.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if errors.Is(err, service.ErrInvalidMetricType) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			logError(err, "Update metric error")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		if err := storeMetrics(s.GetAll(), fileStoragePath); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

func storeMetrics(metrics map[string]model.Metrics, filename string) error {
	metricSlice := make([]model.Metrics, 0, len(metrics))
	for _, m := range metrics {
		metricSlice = append(metricSlice, m)
	}

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	data, err := json.Marshal(metricSlice)
	if err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	return nil
}

func buildMetric(mType, mName, mValue string) (model.Metrics, error) {
	m := model.Metrics{}
	switch mType {
	case model.Counter:
		v, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			return model.Metrics{}, fmt.Errorf("%w: %s", service.ErrInvalidMetricValue, mValue)
		}
		m.ID = mName
		m.MType = mType
		m.Delta = &v
	case model.Gauge:
		v, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			return model.Metrics{}, fmt.Errorf("%w: %s", service.ErrInvalidMetricValue, mValue)
		}
		m.ID = mName
		m.MType = mType
		m.Value = &v
	default:
		return model.Metrics{}, fmt.Errorf("%w: %s", service.ErrUnsupportedMetricType, mType)
	}
	return m, nil
}

func updateHandler(s Storage, fileStoragePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var metric model.Metrics
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&metric); err != nil {
			http.Error(w, fmt.Sprintf("cannot decode request JSON body: %s", err.Error()), http.StatusBadRequest)
			return
		}
		err := s.UpdateMetric(metric)
		if err != nil {
			if errors.Is(err, service.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if errors.Is(err, service.ErrUnsupportedMetricType) ||
				errors.Is(err, service.ErrInvalidMetricType) ||
				errors.Is(err, service.ErrInvalidMetricValue) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			logError(err, "Update metric error")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		if err := storeMetrics(s.GetAll(), fileStoragePath); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

func getMetricHandler(s Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mType := chi.URLParam(r, "mType")
		mName := chi.URLParam(r, "mName")

		switch mType {
		case model.Counter:
			metric, err := s.GetMetric(mType, mName)
			if errors.Is(err, service.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err := fmt.Fprintf(w, "%v", *metric.Delta); err != nil {
				logWriteResponseError(err)
			}
		case model.Gauge:
			metric, err := s.GetMetric(mType, mName)
			if errors.Is(err, service.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err = fmt.Fprintf(w, "%v", *metric.Value); err != nil {
				logWriteResponseError(err)
			}
		default:
			http.Error(w, fmt.Sprintf("unsupported metric type: %s", mType), http.StatusBadRequest)
			return
		}
	}
}

func getHandler(s Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var metric model.Metrics
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&metric); err != nil {
			http.Error(w, fmt.Sprintf("cannot decode request JSON body: %s", err.Error()), http.StatusBadRequest)
			return
		}
		if metric.MType != model.Counter && metric.MType != model.Gauge {
			http.Error(w, fmt.Sprintf("unsupported metric type: %s", metric.MType), http.StatusBadRequest)
			return
		}

		metric, err := s.GetMetric(metric.MType, metric.ID)
		if errors.Is(err, service.ErrMetricNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(metric); err != nil {
			logWriteResponseError(err)
		}
	}
}

func getMetricListHandler(s Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Counters []model.Metrics
			Gauges   []model.Metrics
		}
		metrics := s.GetAll()
		metricNames := make([]string, 0, len(metrics))
		for n := range metrics {
			metricNames = append(metricNames, n)
		}
		slices.Sort(metricNames)

		for _, n := range metricNames {
			m := metrics[n]
			if m.MType == model.Counter {
				data.Counters = append(data.Counters, m)
			}
			if m.MType == model.Gauge {
				data.Gauges = append(data.Gauges, m)
			}
		}
		t := template.Must(template.New("metricList").Parse(`
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Список метрик</title>
</head>
<body>
	<p>Counters:</p>
	{{if .Counters}}
		<ul>
		{{range .Counters}}
			<li>{{.ID}}: {{.Delta}}</li>
		{{end}}
		</ul>
	{{else}}
        <i>No counters yet</i>
	{{end}}
	<p>Gauges:</p>
    {{if .Gauges}}
		<ul>
		{{range .Gauges}}
			<li>{{.ID}}: {{.Value}}</li>
		{{end}}
		</ul>
	{{else}}
		<i>No gauges yet</i>
	{{end}}
</body>
</html>
`))
		w.Header().Set("Content-Type", "text/html")
		if err := t.Execute(w, data); err != nil {
			logWriteResponseError(err)
		}
	}
}

func logWriteResponseError(err error) {
	logError(err, "Error during writing response")
}

func logError(err error, msg string) {
	log.Error().Str("error", err.Error()).Msg(msg)
}
