package repository

import (
	"errors"
)

var ErrMetricNotFound = errors.New("metric not found")
var ErrInvalidMetricValue = errors.New("invalid metric value")
var ErrInvalidMetricType = errors.New("invalid metric type")
var ErrUnsupportedMetricType = errors.New("unsupported metric type")
