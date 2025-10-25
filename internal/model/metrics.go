package model

const (
	Counter = "counter"
	Gauge   = "gauge"
)

type Metric struct {
	ID    string `json:"id"`
	MType string `json:"type"`
	Hash  string `json:"hash,omitempty"`
}

type CounterMetric struct {
	Metric
	Delta int64 `json:"delta"`
}

type GaugeMetric struct {
	Metric
	Value float64 `json:"value"`
}
