package model

const (
	// Counter - тип метрики "счетчик".
	Counter = "counter"
	// Gauge - тип метрики "датчик".
	Gauge = "gauge"
)

// Metrics - модель, описывающая метрику.
//
// ID - наименование (идентификатор) метрики.
//
// MType - тип метрики: счетчик или датчик.
//
// Delta - значение для счетчика (nil для датчика).
//
// Value - значение для датчика (nil для счетчика).
//
// Hash - хэш, опционально используемый для подписи запросов/ответов.
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}
