package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/repository"
)

// Storage абстрагирует хранилище метрик, используемое хендлерами.
type Storage interface {
	UpdateMetric(ctx context.Context, m model.Metrics) error
	UpdateMetrics(ctx context.Context, metrics []model.Metrics) error
	GetMetric(ctx context.Context, mType, mName string) (model.Metrics, error)
	GetAll(ctx context.Context) (map[string]model.Metrics, error)
	Ping(ctx context.Context) error
	Close() error
}

// AuditPublisher публикует события аудита об успешных обновлениях метрик.
type AuditPublisher interface {
	PublishLog(ctx context.Context, now time.Time, ip string, models ...model.Metrics)
}

// MetricRouter собирает и возвращает HTTP-роутер сервиса метрик.
//
// Доступные ручки:
//
//	POST /update/{type}/{name}/{value}
//	GET  /value/{type}/{name}
//	GET  /
//	POST /update
//	POST /updates
//	POST /value
//	GET  /ping
//
// Если key не пустой, включается middleware подписи запросов/ответов.
func MetricRouter(s Storage, a AuditPublisher, key string, logger zerolog.Logger) chi.Router {
	r := chi.NewRouter()
	r.Use(
		middleware.StripSlashes,
		middleware.RealIP,
		withLogging(logger),
		withGzip(logger),
	)
	if key != "" {
		r.Use(withSignature(key, logger))
	}
	textPlainContentType := middleware.AllowContentType("text/plain")
	r.With(textPlainContentType).Post(
		"/update/{mType}/{mName}/{mValue}",
		updateMetricHandler(s, a, logger),
	)
	r.With(textPlainContentType).Get("/value/{mType}/{mName}", getMetricHandler(s, logger))
	r.With(textPlainContentType).Get("/", getMetricListHandler(s, logger))

	applicationJSONContentType := middleware.AllowContentType("application/json")
	r.With(applicationJSONContentType).Post("/update", updateHandler(s, a, logger))
	r.With(applicationJSONContentType).Post("/updates", updatesHandler(s, a, logger))
	r.With(applicationJSONContentType).Post("/value", getHandler(s, logger))

	r.Get("/ping", pingHandler(s, logger))
	return r
}

func withLogging(logger zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			URI := r.RequestURI
			method := r.Method
			start := time.Now()

			responseData := &responseData{
				status: http.StatusOK,
				size:   0,
			}
			lw := newLoggingResponseWriter(w, responseData)
			next.ServeHTTP(lw, r)

			duration := time.Since(start)
			logger.Info().
				Str("uri", URI).
				Str("method", method).
				Str("duration", duration.String()).
				Str("status", strconv.Itoa(responseData.status)).
				Str("size", strconv.Itoa(responseData.size)).
				Msg("Request processed")
		})
	}
}

func withSignature(key string, logger zerolog.Logger) func(next http.Handler) http.Handler {
	const signatureHeaderName = "HashSHA256"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("unable to read request body: %s", err.Error()), http.StatusBadRequest)
				return
			}
			if len(body) > 0 {
				if r.Header.Get(signatureHeaderName) == "" {
					http.Error(w, fmt.Sprintf("header %s is not provided", signatureHeaderName), http.StatusBadRequest)
					return
				}
				err = validateSignature(r.Header.Get(signatureHeaderName), body, key)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			ow := newSigningResponseWriter(w, key, logger)
			next.ServeHTTP(ow, r)
		})
	}
}

func validateSignature(signature string, body []byte, key string) error {
	decodedSig, err := hex.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("unable to decode signature string: %w", err)
	}
	if !hmac.Equal(generateSignature(body, key), decodedSig) {
		return errors.New("invalid request signature")
	}

	return nil
}

func generateSignature(data []byte, key string) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return h.Sum(nil)
}

func withGzip(logger zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ow := w
			acceptEncoding := r.Header.Get("Accept-Encoding")
			supportGzip := strings.Contains(acceptEncoding, "gzip")
			if supportGzip {
				cw := newCompressResponseWriter(w)
				defer cw.Close()
				ow = cw
			}

			contentEncoding := r.Header.Get("Content-Encoding")
			sendsGzip := strings.Contains(contentEncoding, "gzip")
			if sendsGzip {
				cr, err := newCompressRequestReader(r.Body)
				if err != nil {
					if errors.Is(err, gzip.ErrChecksum) || errors.Is(err, gzip.ErrHeader) {
						ow.WriteHeader(http.StatusBadRequest)
						return
					}
					ow.WriteHeader(http.StatusInternalServerError)
					logError(err, "Request error", logger)
					return
				}
				defer cr.Close()
				r.Body = cr
			}
			next.ServeHTTP(ow, r)
		})
	}
}

func updateMetricHandler(s Storage, a AuditPublisher, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mType := chi.URLParam(r, "mType")
		mName := chi.URLParam(r, "mName")
		mValue := chi.URLParam(r, "mValue")

		m, err := buildMetric(mType, mName, mValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = s.UpdateMetric(r.Context(), m)
		if err != nil {
			if errors.Is(err, repository.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if errors.Is(err, repository.ErrInvalidMetricType) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			logError(err, "Update metric error", logger)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

		a.PublishLog(r.Context(), time.Now(), r.RemoteAddr, m)
	}
}

func buildMetric(mType, mName, mValue string) (model.Metrics, error) {
	m := model.Metrics{}
	switch mType {
	case model.Counter:
		v, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			return model.Metrics{}, fmt.Errorf("%w: %s", repository.ErrInvalidMetricValue, mValue)
		}
		m.ID = mName
		m.MType = mType
		m.Delta = &v
	case model.Gauge:
		v, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			return model.Metrics{}, fmt.Errorf("%w: %s", repository.ErrInvalidMetricValue, mValue)
		}
		m.ID = mName
		m.MType = mType
		m.Value = &v
	default:
		return model.Metrics{}, fmt.Errorf("%w: %s", repository.ErrUnsupportedMetricType, mType)
	}
	return m, nil
}

func updateHandler(s Storage, a AuditPublisher, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var metric model.Metrics
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&metric); err != nil {
			http.Error(w, fmt.Sprintf("cannot decode request JSON body: %s", err.Error()), http.StatusBadRequest)
			return
		}
		err := s.UpdateMetric(r.Context(), metric)
		if err != nil {
			if errors.Is(err, repository.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if errors.Is(err, repository.ErrUnsupportedMetricType) ||
				errors.Is(err, repository.ErrInvalidMetricType) ||
				errors.Is(err, repository.ErrInvalidMetricValue) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			logError(err, "Update metric error", logger)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

		a.PublishLog(r.Context(), time.Now(), r.RemoteAddr, metric)
	}
}

func updatesHandler(s Storage, a AuditPublisher, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var metrics []model.Metrics
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&metrics); err != nil {
			http.Error(w, fmt.Sprintf("cannot decode request JSON body: %s", err.Error()), http.StatusBadRequest)
			return
		}
		err := s.UpdateMetrics(r.Context(), metrics)
		if err != nil {
			if errors.Is(err, repository.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if errors.Is(err, repository.ErrUnsupportedMetricType) ||
				errors.Is(err, repository.ErrInvalidMetricType) ||
				errors.Is(err, repository.ErrInvalidMetricValue) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			logError(err, "Update metrics error", logger)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

		a.PublishLog(r.Context(), time.Now(), r.RemoteAddr, metrics...)
	}
}

func getMetricHandler(s Storage, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mType := chi.URLParam(r, "mType")
		mName := chi.URLParam(r, "mName")

		switch mType {
		case model.Counter:
			metric, err := s.GetMetric(r.Context(), mType, mName)
			if errors.Is(err, repository.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err := fmt.Fprintf(w, "%v", *metric.Delta); err != nil {
				logWriteResponseError(err, logger)
			}
		case model.Gauge:
			metric, err := s.GetMetric(r.Context(), mType, mName)
			if errors.Is(err, repository.ErrMetricNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err = fmt.Fprintf(w, "%v", *metric.Value); err != nil {
				logWriteResponseError(err, logger)
			}
		default:
			http.Error(w, fmt.Sprintf("unsupported metric type: %s", mType), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func getHandler(s Storage, logger zerolog.Logger) http.HandlerFunc {
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

		metric, err := s.GetMetric(r.Context(), metric.MType, metric.ID)
		if errors.Is(err, repository.ErrMetricNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(metric); err != nil {
			logWriteResponseError(err, logger)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func getMetricListHandler(s Storage, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Counters []model.Metrics
			Gauges   []model.Metrics
		}
		metrics, err := s.GetAll(r.Context())
		if err != nil {
			logError(err, "Get metric list error", logger)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
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
			logWriteResponseError(err, logger)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func logWriteResponseError(err error, logger zerolog.Logger) {
	logError(err, "Error during writing response", logger)
}

func logError(err error, msg string, logger zerolog.Logger) {
	logger.Error().Err(err).Msg(msg)
}

func pingHandler(s Storage, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := s.Ping(ctx); err != nil {
			logError(err, "Storage connection error", logger)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
