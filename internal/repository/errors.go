package repository

import (
	"errors"
)

// ErrMetricNotFound возвращается, когда запрошенная метрика не найдена.
var ErrMetricNotFound = errors.New("metric not found")

// ErrInvalidMetricValue возвращается, когда метрика содержит некорректное значение.
var ErrInvalidMetricValue = errors.New("invalid metric value")

// ErrInvalidMetricType возвращается, когда тип метрики не совпадает с сохраненным типом.
var ErrInvalidMetricType = errors.New("invalid metric type")

// ErrUnsupportedMetricType возвращается, когда тип метрики не поддерживается.
var ErrUnsupportedMetricType = errors.New("unsupported metric type")
