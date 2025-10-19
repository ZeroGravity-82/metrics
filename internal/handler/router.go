package handler

import (
	"net/http"
	"strconv"
	"strings"

	"zerogravity-82/metrics/internal/model"
	"zerogravity-82/metrics/internal/service"
)

func UpdateMetricHandler(ms service.MemStorage) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "text/plain" {
			http.Error(w, "Unsupported media type", http.StatusUnsupportedMediaType)
			return
		}

		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) != 5 {
			http.Error(w, "404 page not found", http.StatusNotFound)
			return
		}

		metricValue := pathParts[4]
		valueInt64, errInt64 := strconv.ParseInt(metricValue, 10, 64)
		valueFloat64, errFloat64 := strconv.ParseFloat(metricValue, 64)
		var delta *int64
		if errInt64 == nil {
			delta = &valueInt64
		}
		var value *float64
		if errFloat64 == nil {
			value = &valueFloat64
		}
		metric := model.Metric{
			ID:    pathParts[3],
			MType: pathParts[2],
			Delta: delta,
			Value: value,
			Hash:  "", // Пока не понятно, как и где формируется этот хэш
		}

		err := ms.UpdateMetric(metric)
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
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
}
