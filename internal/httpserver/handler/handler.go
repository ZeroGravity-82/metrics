package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/repository"
)

// Storage абстрагирует хранилище метрик.
type Storage interface {
	UpdateMetric(ctx context.Context, m model.Metrics) error
	UpdateMetrics(ctx context.Context, metrics []model.Metrics) error
	GetMetric(ctx context.Context, mType, mName string) (model.Metrics, error)
	GetAll(ctx context.Context) (map[string]model.Metrics, error)
	Ping(ctx context.Context) error
}

// AuditPublisher публикует события аудита об успешных обновлениях метрик.
type AuditPublisher interface {
	PublishLog(ctx context.Context, now time.Time, ip string, models ...model.Metrics)
}

// Handler содержит зависимости HTTP-хендлеров сервиса метрик.
type Handler struct {
	s      Storage
	a      AuditPublisher
	logger zerolog.Logger
}

// New создаёт Handler с нужными зависимостями.
func New(s Storage, a AuditPublisher, logger zerolog.Logger) *Handler {
	return &Handler{s: s, a: a, logger: logger}
}

func (h *Handler) updateMetric(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "mType")
	mName := chi.URLParam(r, "mName")
	mValue := chi.URLParam(r, "mValue")

	m, err := buildMetric(mType, mName, mValue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.s.UpdateMetric(r.Context(), m)
	if err != nil {
		if errors.Is(err, repository.ErrMetricNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if errors.Is(err, repository.ErrInvalidMetricType) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		logError(err, "Update metric error", h.logger)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	h.a.PublishLog(r.Context(), time.Now(), r.RemoteAddr, m)
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

func logError(err error, msg string, logger zerolog.Logger) {
	logger.Error().Err(err).Msg(msg)
}

func (h *Handler) getMetric(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "mType")
	mName := chi.URLParam(r, "mName")

	switch mType {
	case model.Counter:
		metric, err := h.s.GetMetric(r.Context(), mType, mName)
		if errors.Is(err, repository.ErrMetricNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if _, err := fmt.Fprintf(w, "%v", *metric.Delta); err != nil {
			logWriteResponseError(err, h.logger)
		}
	case model.Gauge:
		metric, err := h.s.GetMetric(r.Context(), mType, mName)
		if errors.Is(err, repository.ErrMetricNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if _, err = fmt.Fprintf(w, "%v", *metric.Value); err != nil {
			logWriteResponseError(err, h.logger)
		}
	default:
		http.Error(w, fmt.Sprintf("unsupported metric type: %s", mType), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func logWriteResponseError(err error, logger zerolog.Logger) {
	logError(err, "Error during writing response", logger)
}

func (h *Handler) getMetricList(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Counters []model.Metrics
		Gauges   []model.Metrics
	}
	metrics, err := h.s.GetAll(r.Context())
	if err != nil {
		logError(err, "Get metric list error", h.logger)
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
		logWriteResponseError(err, h.logger)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var metric model.Metrics
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&metric); err != nil {
		http.Error(w, fmt.Sprintf("cannot decode request JSON body: %s", err.Error()), http.StatusBadRequest)
		return
	}
	err := h.s.UpdateMetric(r.Context(), metric)
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
		logError(err, "Update metric error", h.logger)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	h.a.PublishLog(r.Context(), time.Now(), r.RemoteAddr, metric)
}

func (h *Handler) updates(w http.ResponseWriter, r *http.Request) {
	var metrics []model.Metrics
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&metrics); err != nil {
		http.Error(w, fmt.Sprintf("cannot decode request JSON body: %s", err.Error()), http.StatusBadRequest)
		return
	}
	err := h.s.UpdateMetrics(r.Context(), metrics)
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
		logError(err, "Update metrics error", h.logger)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	h.a.PublishLog(r.Context(), time.Now(), r.RemoteAddr, metrics...)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
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

	metric, err := h.s.GetMetric(r.Context(), metric.MType, metric.ID)
	if errors.Is(err, repository.ErrMetricNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(metric); err != nil {
		logWriteResponseError(err, h.logger)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.s.Ping(ctx); err != nil {
		logError(err, "Storage connection error", h.logger)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
