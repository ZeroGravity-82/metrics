package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

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
			_, _ = io.WriteString(w, fmt.Sprintf("%v", metric.Delta))
		case model.Gauge:
			metric, err := s.GetGaugeMetric(mName)
			if mnfe, ok := err.(service.MetricNotFoundError); ok {
				http.Error(w, mnfe.Error(), http.StatusNotFound)
				return
			}
			_, _ = io.WriteString(w, fmt.Sprintf("%v", metric.Value))
		default:
			http.Error(w, "unsupported metric type", http.StatusBadRequest)
			return
		}
	}
}

func getMetricListHandler(s service.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		counters, gauges := s.GetAll()
		html := buildHTML(counters, gauges)
		_, _ = io.WriteString(w, html)
	}
}

func buildHTML(counters []model.CounterMetric, gauges []model.GaugeMetric) string {
	var counterList, gaugeList string
	for _, c := range counters {
		counterList += fmt.Sprintf("<li>%s: %v</li>", c.ID, c.Delta)
	}
	for _, g := range gauges {
		gaugeList += fmt.Sprintf("<li>%s: %v</li>", g.ID, g.Value)
	}
	if counterList == "" {
		counterList = `<i>No counters yet</i>`
	}
	if gaugeList == "" {
		gaugeList = `<i>No gauges yet</i>`
	}
	html := getMetricListHTMLTemplate()
	html = strings.Replace(html, "{{counterList}}", counterList, 1)
	html = strings.Replace(html, "{{gaugeList}}", gaugeList, 1)
	return html
}

func getMetricListHTMLTemplate() string {
	return `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Список метрик</title>
</head>
<body>
	<p>Counters:</p>
    <ul>{{counterList}}</ul>
	<p>Gauges:</p>
	<ul>{{gaugeList}}</ul>
</body>
</html>`
}
