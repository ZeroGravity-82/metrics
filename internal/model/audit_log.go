package model

// AuditLog - модель, описывающая событие аудита запроса.
//
// TS содержит unix timestamp события.
//
// Metrics содержит наименования полученных метрик.
//
// IP содержит IP-адрес входящего запроса.
type AuditLog struct {
	TS      int64    `json:"ts"`
	Metrics []string `json:"metrics"`
	IP      string   `json:"ip_address"`
}
