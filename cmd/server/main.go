package main

import (
	"log"
	"net/http"

	"zerogravity-82/metrics/internal/handler"
	"zerogravity-82/metrics/internal/service"
)

func main() {
	ms := service.NewMemStorage()

	mux := http.NewServeMux()
	mux.Handle("/update/", handler.UpdateMetricHandler(ms))

	log.Fatal(http.ListenAndServe("localhost:8080", mux))
}
