package agent

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"time"

	"github.com/go-resty/resty/v2"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/model"
)

type metrics struct {
	memStat     map[string]any
	pollCount   uint64
	randomValue uint32
}

func Run(cfg config.Config) {
	m := metrics{}
	m.memStat = make(map[string]any)

	httpClient := resty.New()
	lastSentTime := time.Now()
	for {
		pollMetrics(&m)
		time.Sleep(time.Duration(*cfg.PollInterval) * time.Second)

		if time.Since(lastSentTime) >= time.Duration(*cfg.ReportInterval)*time.Second {
			lastSentTime = time.Now()
			sendReport(*cfg.ServerAddr, &m, httpClient)
		}
	}
}

func pollMetrics(m *metrics) {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)

	m.memStat["Alloc"] = memStats.Alloc
	m.memStat["BuckHashSys"] = memStats.BuckHashSys
	m.memStat["Frees"] = memStats.Frees
	m.memStat["GCCPUFraction"] = memStats.GCCPUFraction
	m.memStat["GCSys"] = memStats.GCSys
	m.memStat["HeapAlloc"] = memStats.HeapAlloc
	m.memStat["HeapIdle"] = memStats.HeapIdle
	m.memStat["HeapInuse"] = memStats.HeapInuse
	m.memStat["HeapObjects"] = memStats.HeapObjects
	m.memStat["HeapReleased"] = memStats.HeapReleased
	m.memStat["HeapSys"] = memStats.HeapSys
	m.memStat["LastGC"] = memStats.LastGC
	m.memStat["Lookups"] = memStats.Lookups
	m.memStat["MCacheInuse"] = memStats.MCacheInuse
	m.memStat["MCacheSys"] = memStats.MCacheSys
	m.memStat["MSpanInuse"] = memStats.MSpanInuse
	m.memStat["MSpanSys"] = memStats.MSpanSys
	m.memStat["Mallocs"] = memStats.Mallocs
	m.memStat["NextGC"] = memStats.NextGC
	m.memStat["NumForcedGC"] = memStats.NumForcedGC
	m.memStat["NumGC"] = memStats.NumGC
	m.memStat["OtherSys"] = memStats.OtherSys
	m.memStat["PauseTotalNs"] = memStats.PauseTotalNs
	m.memStat["StackInuse"] = memStats.StackInuse
	m.memStat["StackSys"] = memStats.StackSys
	m.memStat["Sys"] = memStats.Sys
	m.memStat["TotalAlloc"] = memStats.TotalAlloc
	m.pollCount++
	m.randomValue = rand.Uint32()
}

func sendReport(serverAddr string, m *metrics, httpClient *resty.Client) {
	for name, value := range m.memStat {
		err := sendMetric(serverAddr, model.Gauge, name, value, httpClient)
		if err != nil {
			logError(model.Gauge, name, value, err)
		}
	}

	err := sendMetric(serverAddr, model.Counter, "PollCount", m.pollCount, httpClient)
	if err != nil {
		logError(model.Gauge, "PollCount", m.pollCount, err)
	}
	m.pollCount = 0

	err = sendMetric(serverAddr, model.Gauge, "RandomValue", m.randomValue, httpClient)
	if err != nil {
		logError(model.Gauge, "RandomValue", m.randomValue, err)
	}
}

func sendMetric(serverAddr, mType, mName string, mValue any, httpClient *resty.Client) error {
	url := fmt.Sprintf("%s/update/%s/%s/%v", serverAddr, mType, mName, mValue)
	_, err := httpClient.R().SetHeader("Content-Type", "text/plain").Post(url)
	if err != nil {
		return err
	}
	return nil
}

func logError(mType, mName string, mValue any, err error) {
	log.Printf("Error on sending metric '%s' of type '%s' with value '%d': %v", mName, mType, mValue, err)
}
