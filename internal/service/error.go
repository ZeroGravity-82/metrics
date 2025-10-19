package service

type MetricNotFoundError struct {
	Message string
}

func (e MetricNotFoundError) Error() string {
	return e.Message
}

type InvalidMetricValueError struct {
	Message string
}

func (e InvalidMetricValueError) Error() string {
	return e.Message
}

type UnsupportedMetricTypeError struct {
	Message string
}

func (e UnsupportedMetricTypeError) Error() string {
	return e.Message
}
