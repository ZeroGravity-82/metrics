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
	cfg, err := config.GetServerConfig()
	if err != nil {
		log.Fatal(err)
	}
	ms := service.NewMemStorage()
	log.Printf("server started on %s...", cfg.ServerAddr)
	err = http.ListenAndServe(cfg.ServerAddr, handler.MetricRouter(ms))
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
