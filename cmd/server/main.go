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
	serverAddr := *cfg.ServerAddr
	log.Printf("Server started at %s...", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, handler.MetricRouter(ms)))
}
