package handler

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/service"
)

type Storage interface {
	UpdateMetric(mType, mName, mValue string) error
	GetMetric(mType, mName string) (model.Metrics, error)
	GetAll() map[string]model.Metrics
}

func MetricRouter(s Storage) chi.Router {
	r := chi.NewRouter()
	r.Use(
		middleware.AllowContentType("text/plain"),
		withLogging,
	)
	r.Post("/update/{mType}/{mName}/{mValue}", updateMetricHandler(s))
	r.Get("/value/{mType}/{mName}", getMetricHandler(s))
	r.Get("/", getMetricListHandler(s))
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

func updateMetricHandler(s Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mType := chi.URLParam(r, "mType")
		mName := chi.URLParam(r, "mName")
		mValue := chi.URLParam(r, "mValue")
		err := s.UpdateMetric(mType, mName, mValue)
		if err != nil {
			if errors.Is(err, service.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if errors.Is(err, service.ErrUnsupportedMetricType) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if errors.Is(err, service.ErrInvalidMetricValue) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			logError(err, "Update metric error")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
			if _, err := fmt.Fprintf(w, "%v", *metric.Delta); err != nil {
				logWriteResponseError(err)
			}
		case model.Gauge:
			metric, err := s.GetMetric(mType, mName)
			if errors.Is(err, service.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if _, err = fmt.Fprintf(w, "%v", *metric.Value); err != nil {
				logWriteResponseError(err)
			}
		default:
			http.Error(w, fmt.Sprintf("unsupported metric type: %s", mType), http.StatusBadRequest)
			return
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
