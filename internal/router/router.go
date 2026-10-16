package router

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/peshk1n/site-monitor/internal/handler"
)

func New(h *handler.Handler, ready func(context.Context) error) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		handler.JSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "object unavailable"}})
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { handler.JSON(w, 200, map[string]string{"status": "ok"}) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if ready(ctx) != nil {
			handler.JSON(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		handler.JSON(w, 200, map[string]string{"status": "ready"})
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(h.Authenticate)
		r.Get("/me", h.Me)
		r.Get("/projects", h.Projects)
		r.Post("/projects", h.CreateProject)
		r.Route("/projects/{projectID}", func(r chi.Router) {
			r.Get("/", h.Project)
			r.Patch("/", h.ChangeProject("rename"))
			r.Post("/archive", h.ChangeProject("archive"))
			r.Post("/restore", h.ChangeProject("restore"))
			r.Get("/dashboard", h.Dashboard)
			r.Get("/incidents", h.Incidents)
			r.Get("/monitors", h.Monitors)
			r.Post("/monitors", h.CreateMonitor)
			r.Route("/monitors/{monitorID}", func(r chi.Router) {
				r.Get("/", h.Monitor)
				r.Put("/", h.UpdateMonitor)
				for _, action := range []string{"pause", "resume", "archive", "restore"} {
					r.Post("/"+action, h.ChangeMonitor(action))
				}
				r.Get("/results", h.Results)
				r.Get("/incidents", h.Incidents)
			})
		})
	})
	return r
}
