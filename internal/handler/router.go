package handler

import (
	"net/http"
	"strings"

	"zerogravity-82/metrics/internal/service"
)

func UpdateMetricHandler(s service.Storage) http.Handler {
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

		action := pathParts[1]
		if action != "update" {
			http.Error(w, "Unsupported action", http.StatusNotFound)
			return
		}

		mType := pathParts[2]
		mName := pathParts[3]
		mValue := pathParts[4]
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
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
}
