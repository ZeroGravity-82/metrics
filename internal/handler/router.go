package handler

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/service"
)

func MetricRouter(s service.Storage) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.AllowContentType("text/plain"))
	r.Post("/update/{mType}/{mName}/{mValue}", updateMetricHandler(s))
	r.Get("/value/{mType}/{mName}", getMetricHandler(s))
	r.Get("/", getMetricListHandler(s))
	return r
}

func updateMetricHandler(s service.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mType := chi.URLParam(r, "mType")
		mName := chi.URLParam(r, "mName")
		mValue := chi.URLParam(r, "mValue")
		err := s.UpdateMetric(mType, mName, mValue)
		if err != nil {
			if mnfe, ok := err.(service.MetricNotFoundError); ok {
				http.Error(w, mnfe.Error(), http.StatusNotFound)
				return
			}
			if umte, ok := err.(service.UnsupportedMetricTypeError); ok {
				http.Error(w, umte.Error(), http.StatusBadRequest)
				return
			}
			if imve, ok := err.(service.InvalidMetricValueError); ok {
				http.Error(w, imve.Error(), http.StatusBadRequest)
				return
			}
			log.Printf("update metric error: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

func getMetricHandler(s service.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mType := chi.URLParam(r, "mType")
		mName := chi.URLParam(r, "mName")

		switch mType {
		case model.Counter:
			metric, err := s.GetCounterMetric(mName)
			if mnfe, ok := err.(service.MetricNotFoundError); ok {
				http.Error(w, mnfe.Error(), http.StatusNotFound)
				return
			}
			_, err = io.WriteString(w, fmt.Sprintf("%v", metric.Delta))
			log.Printf("error during writing response: %v", err)
		case model.Gauge:
			metric, err := s.GetGaugeMetric(mName)
			if mnfe, ok := err.(service.MetricNotFoundError); ok {
				http.Error(w, mnfe.Error(), http.StatusNotFound)
				return
			}
			_, err = io.WriteString(w, fmt.Sprintf("%v", metric.Value))
			log.Printf("error during writing response: %v", err)
		default:
			http.Error(w, "unsupported metric type", http.StatusBadRequest)
			return
		}
	}
}

func getMetricListHandler(s service.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Counters []model.CounterMetric
			Gauges   []model.GaugeMetric
		}
		data.Counters, data.Gauges = s.GetAll()
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
			log.Printf("error during writing response: %v", err)
		}
	}
}
