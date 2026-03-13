package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/peshk1n/site-monitor/internal/handler"
)

func NewRouter(
	monitorHandler *handler.MonitorHandler,
	checkHandler *handler.CheckHandler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Route("/api/v1/monitors", func(r chi.Router) {
		r.Get("/", monitorHandler.GetAll)
		r.Post("/", monitorHandler.Create)
		r.Get("/{id}", monitorHandler.GetByID)
		r.Delete("/{id}", monitorHandler.Delete)

		r.Get("/{id}/checks", checkHandler.GetByMonitorID)
		r.Get("/{id}/checks/last", checkHandler.GetLastByMonitorID)
	})
	return r
}
