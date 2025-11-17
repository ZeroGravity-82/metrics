package agent

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"
)

type metrics struct {
	memStat     map[string]float64
	pollCount   int64
	randomValue float64
}

func Run(cfg config.AgentConfig) {
	m := metrics{}
	m.memStat = make(map[string]float64)

	httpClient := resty.New()
	lastSentTime := time.Now()
	for {
		pollMetrics(&m)
		m.pollCount++
		time.Sleep(time.Duration(cfg.PollInterval) * time.Second)

		if time.Since(lastSentTime) >= time.Duration(cfg.ReportInterval)*time.Second {
			lastSentTime = time.Now()
			sendReport(cfg.ServerAddr, &m, httpClient)
			m.pollCount = 0
		}
	}
}

func pollMetrics(m *metrics) {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)

	m.memStat["Alloc"] = float64(memStats.Alloc)
	m.memStat["BuckHashSys"] = float64(memStats.BuckHashSys)
	m.memStat["Frees"] = float64(memStats.Frees)
	m.memStat["GCCPUFraction"] = memStats.GCCPUFraction
	m.memStat["GCSys"] = float64(memStats.GCSys)
	m.memStat["HeapAlloc"] = float64(memStats.HeapAlloc)
	m.memStat["HeapIdle"] = float64(memStats.HeapIdle)
	m.memStat["HeapInuse"] = float64(memStats.HeapInuse)
	m.memStat["HeapObjects"] = float64(memStats.HeapObjects)
	m.memStat["HeapReleased"] = float64(memStats.HeapReleased)
	m.memStat["HeapSys"] = float64(memStats.HeapSys)
	m.memStat["LastGC"] = float64(memStats.LastGC)
	m.memStat["Lookups"] = float64(memStats.Lookups)
	m.memStat["MCacheInuse"] = float64(memStats.MCacheInuse)
	m.memStat["MCacheSys"] = float64(memStats.MCacheSys)
	m.memStat["MSpanInuse"] = float64(memStats.MSpanInuse)
	m.memStat["MSpanSys"] = float64(memStats.MSpanSys)
	m.memStat["Mallocs"] = float64(memStats.Mallocs)
	m.memStat["NextGC"] = float64(memStats.NextGC)
	m.memStat["NumForcedGC"] = float64(memStats.NumForcedGC)
	m.memStat["NumGC"] = float64(memStats.NumGC)
	m.memStat["OtherSys"] = float64(memStats.OtherSys)
	m.memStat["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	m.memStat["StackInuse"] = float64(memStats.StackInuse)
	m.memStat["StackSys"] = float64(memStats.StackSys)
	m.memStat["Sys"] = float64(memStats.Sys)
	m.memStat["TotalAlloc"] = float64(memStats.TotalAlloc)
	m.randomValue = float64(rand.Uint32())
}

func sendReport(serverAddr string, metrics *metrics, httpClient *resty.Client) {
	for name, value := range metrics.memStat {
		m := model.Metrics{ID: name, MType: model.Gauge, Value: &value}
		err := sendMetric(serverAddr, m, httpClient)
		if err != nil {
			logSendReportError(err, m)
		}
	}

	m := model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &metrics.pollCount}
	err := sendMetric(serverAddr, m, httpClient)
	if err != nil {
		logSendReportError(err, m)
	}

	m = model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &metrics.randomValue}
	err = sendMetric(serverAddr, m, httpClient)
	if err != nil {
		logSendReportError(err, m)
	}
}

func sendMetric(serverAddr string, m model.Metrics, httpClient *resty.Client) error {
	serverAddr = addDefaultURLSchema(serverAddr)
	url := fmt.Sprintf("%s/update", serverAddr)
	_, err := httpClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(m).
		Post(url)
	if err != nil {
		return err
	}
	return nil
}

func addDefaultURLSchema(URL string) string {
	if strings.HasPrefix(URL, "https://") || strings.HasPrefix(URL, "http://") {
		return URL
	}
	hp := strings.Split(URL, ":")
	host := hp[0]
	if len(host) == 0 {
		host = "localhost"
	}
	port := hp[1]
	urlPrefix := ""
	if host == "localhost" {
		urlPrefix = "http://"
	} else {
		urlPrefix = "https://"
	}
	return urlPrefix + host + ":" + port
}

func logSendReportError(err error, m model.Metrics) {
	log.Error().
		Str("metric", fmt.Sprintf("%v", m)).
		Str("error", err.Error()).
		Msg("Error on sending metric")
}
