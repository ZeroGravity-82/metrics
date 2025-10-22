package agent

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"zerogravity-82/metrics/internal/model"
)

const (
	serverBaseURL  string        = "http://localhost:8080"
	pollInterval   time.Duration = time.Second * 2
	reportInterval time.Duration = time.Second * 13
)

type metrics struct {
	memStat     map[string]any
	pollCount   uint64
	randomValue uint32
}

func Run() {
	m := metrics{}
	m.memStat = make(map[string]any)

	lastSentTime := time.Now()
	for {
		pollMetrics(&m)
		time.Sleep(pollInterval)

		if time.Now().Sub(lastSentTime) >= reportInterval {
			lastSentTime = time.Now()
			sendReport(serverBaseURL, &m)
			m.pollCount = 0
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

func sendReport(serverBaseURL string, m *metrics) {
	for name, value := range m.memStat {
		err := sendMetric(serverBaseURL, model.Gauge, name, value)
		if err != nil {
			logError(model.Gauge, name, value, err)
		}
	}
	err := sendMetric(serverBaseURL, model.Counter, "PollCount", m.pollCount)
	if err != nil {
		logError(model.Gauge, "PollCount", m.pollCount, err)
	}
	err = sendMetric(serverBaseURL, model.Gauge, "RandomValue", m.randomValue)
	if err != nil {
		logError(model.Gauge, "RandomValue", m.randomValue, err)
	}
}

func sendMetric(serverBaseURL, mType, mName string, mValue any) error {
	url := fmt.Sprintf("%s/update/%s/%s/%v", serverBaseURL, mType, mName, mValue)
	response, err := http.Post(url, "text/plain", http.NoBody)
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func logError(mType, mName string, mValue any, err error) {
	log.Printf("Error on sending metric '%s' of type '%s' with value '%d': %v", mName, mType, mValue, err)
}
