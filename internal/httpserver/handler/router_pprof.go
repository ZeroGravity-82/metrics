package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// PprofRouter собирает роутер с подключенными обработчиками pprof.
func PprofRouter() chi.Router {
	r := chi.NewRouter()
	r.Mount("/debug", middleware.Profiler())
	return r
}
