package main

import (
	"errors"
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
	log.Printf("server started on %s...", serverAddr)
	err := http.ListenAndServe(serverAddr, handler.MetricRouter(ms))
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
