package main

import (
	"log"
	"net/http"

	"zerogravity-82/metrics/internal/config"
	"zerogravity-82/metrics/internal/handler"
	"zerogravity-82/metrics/internal/service"
)

func main() {
	cfg := config.ParseFlags()
	ms := service.NewMemStorage()
	log.Fatal(http.ListenAndServe(*cfg.ServerAddr, handler.MetricRouter(ms)))
}
