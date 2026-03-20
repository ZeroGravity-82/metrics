package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func PprofRouter() chi.Router {
	r := chi.NewRouter()
	r.Mount("/debug", middleware.Profiler())
	return r
}
