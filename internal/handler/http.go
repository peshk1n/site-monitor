package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type Authenticator interface {
	Authenticate(context.Context, string) (models.User, error)
}
type userKey struct{}
type Handler struct {
	app  *service.Application
	auth Authenticator
	log  *slog.Logger
}

func New(app *service.Application, auth Authenticator, log *slog.Logger) *Handler {
	return &Handler{app: app, auth: auth, log: log}
}
func User(ctx context.Context) models.User {
	user, _ := ctx.Value(userKey{}).(models.User)
	return user
}
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := h.auth.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			h.fail(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}
func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func (h *Handler) fail(w http.ResponseWriter, err error) {
	code, message, status := "internal_error", "internal server error", http.StatusInternalServerError
	var validation *service.ValidationError
	switch {
	case errors.As(err, &validation):
		code = "invalid_request"
		message = validation.Message
		status = 400
	case errors.Is(err, service.ErrUnauthorized):
		code = "unauthorized"
		message = err.Error()
		status = 401
	case errors.Is(err, service.ErrNotFound):
		code = "not_found"
		message = err.Error()
		status = 404
	case errors.Is(err, service.ErrForbidden):
		code = "forbidden"
		message = err.Error()
		status = 403
	case errors.Is(err, service.ErrConflict):
		code = "conflict"
		message = err.Error()
		status = 409
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
		message = "request timed out"
		status = 504
	default:
		h.log.Error("request failed", "error", err)
	}
	JSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func decode(w http.ResponseWriter, r *http.Request, dest any) error {
	if media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || media != "application/json" {
		return service.Invalid("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dest); err != nil {
		return service.Invalid("invalid JSON body")
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return service.Invalid("expected a single JSON object")
	}
	return nil
}
func id(r *http.Request, name string) (int64, error) {
	value, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || value <= 0 {
		return 0, service.Invalid("invalid " + name)
	}
	return value, nil
}
func page(r *http.Request) (models.Page, error) {
	p := models.Page{Limit: 20}
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		p.Limit, err = strconv.Atoi(raw)
		if err != nil {
			return p, service.Invalid("invalid limit")
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		p.Offset, err = strconv.Atoi(raw)
		if err != nil {
			return p, service.Invalid("invalid offset")
		}
	}
	if !service.ValidPage(p) {
		return p, service.Invalid("invalid pagination")
	}
	return p, nil
}
func archived(r *http.Request) (bool, error) {
	raw := r.URL.Query().Get("archived")
	if raw == "" || raw == "false" {
		return false, nil
	}
	if raw == "true" {
		return true, nil
	}
	return false, service.Invalid("archived must be true or false")
}
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) { JSON(w, 200, User(r.Context())) }
func (h *Handler) Projects(w http.ResponseWriter, r *http.Request) {
	p, err := page(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	archive, err := archived(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	items, err := h.app.Projects(r.Context(), User(r.Context()).ID, archive, p)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, map[string]any{"projects": items, "limit": p.Limit, "offset": p.Offset})
}
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if !service.ValidRequestKey(key) {
		h.fail(w, service.Invalid("Idempotency-Key must be a UUID"))
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decode(w, r, &body); err != nil {
		h.fail(w, err)
		return
	}
	project, err := h.app.CreateProject(r.Context(), User(r.Context()).ID, body.Name, key)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 201, project)
}
func (h *Handler) Project(w http.ResponseWriter, r *http.Request) {
	p, err := id(r, "projectID")
	if err != nil {
		h.fail(w, err)
		return
	}
	project, err := h.app.Project(r.Context(), User(r.Context()).ID, p)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, project)
}
func (h *Handler) ChangeProject(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := id(r, "projectID")
		if err != nil {
			h.fail(w, err)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if action == "rename" {
			if err = decode(w, r, &body); err != nil {
				h.fail(w, err)
				return
			}
		}
		project, err := h.app.ChangeProject(r.Context(), User(r.Context()).ID, p, action, body.Name)
		if err != nil {
			h.fail(w, err)
			return
		}
		JSON(w, 200, project)
	}
}
func (h *Handler) Monitors(w http.ResponseWriter, r *http.Request) {
	p, err := id(r, "projectID")
	if err != nil {
		h.fail(w, err)
		return
	}
	pg, err := page(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	archive, err := archived(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	items, err := h.app.Monitors(r.Context(), User(r.Context()).ID, p, models.MonitorFilter{Archived: archive, Search: r.URL.Query().Get("search"), Category: r.URL.Query().Get("category"), Page: pg})
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, map[string]any{"monitors": items, "limit": pg.Limit, "offset": pg.Offset})
}
func (h *Handler) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if !service.ValidRequestKey(key) {
		h.fail(w, service.Invalid("Idempotency-Key must be a UUID"))
		return
	}
	p, err := id(r, "projectID")
	if err != nil {
		h.fail(w, err)
		return
	}
	body := models.DefaultMonitorConfig()
	if err = decode(w, r, &body); err != nil {
		h.fail(w, err)
		return
	}
	m, err := h.app.CreateMonitor(r.Context(), User(r.Context()).ID, p, body, key)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 201, m)
}
func coordinates(r *http.Request) (int64, int64, error) {
	p, err := id(r, "projectID")
	if err != nil {
		return 0, 0, err
	}
	m, err := id(r, "monitorID")
	return p, m, err
}
func (h *Handler) Monitor(w http.ResponseWriter, r *http.Request) {
	p, m, err := coordinates(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	result, err := h.app.Monitor(r.Context(), User(r.Context()).ID, p, m)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, result)
}
func (h *Handler) UpdateMonitor(w http.ResponseWriter, r *http.Request) {
	p, m, err := coordinates(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	var body struct {
		models.MonitorConfig
		ConfigVersion int64 `json:"config_version"`
	}
	if err = decode(w, r, &body); err != nil {
		h.fail(w, err)
		return
	}
	result, err := h.app.UpdateMonitor(r.Context(), User(r.Context()).ID, p, m, body.ConfigVersion, body.MonitorConfig)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, result)
}
func (h *Handler) ChangeMonitor(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, m, err := coordinates(r)
		if err != nil {
			h.fail(w, err)
			return
		}
		var body struct {
			ConfigVersion int64 `json:"config_version"`
		}
		if err = decode(w, r, &body); err != nil {
			h.fail(w, err)
			return
		}
		result, err := h.app.ChangeMonitor(r.Context(), User(r.Context()).ID, p, m, body.ConfigVersion, action)
		if err != nil {
			h.fail(w, err)
			return
		}
		JSON(w, 200, result)
	}
}
func (h *Handler) Results(w http.ResponseWriter, r *http.Request) {
	p, m, err := coordinates(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	pg, err := page(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	items, err := h.app.Results(r.Context(), User(r.Context()).ID, p, m, pg)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, map[string]any{"results": items, "limit": pg.Limit, "offset": pg.Offset})
}
func (h *Handler) Incidents(w http.ResponseWriter, r *http.Request) {
	p, err := id(r, "projectID")
	if err != nil {
		h.fail(w, err)
		return
	}
	pg, err := page(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	var monitor *int64
	if chi.URLParam(r, "monitorID") != "" {
		value, err := id(r, "monitorID")
		if err != nil {
			h.fail(w, err)
			return
		}
		monitor = &value
	}
	items, err := h.app.Incidents(r.Context(), User(r.Context()).ID, p, monitor, pg)
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, map[string]any{"incidents": items, "limit": pg.Limit, "offset": pg.Offset})
}
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	p, err := id(r, "projectID")
	if err != nil {
		h.fail(w, err)
		return
	}
	to := time.Now().UTC()
	span := 24 * time.Hour
	switch r.URL.Query().Get("period") {
	case "", "24h":
	case "7d":
		span = 7 * 24 * time.Hour
	case "30d":
		span = 30 * 24 * time.Hour
	default:
		h.fail(w, service.Invalid("period must be 24h, 7d or 30d"))
		return
	}
	var monitor *int64
	if raw := r.URL.Query().Get("monitor_id"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			h.fail(w, service.Invalid("invalid monitor_id"))
			return
		}
		monitor = &value
	}
	result, err := h.app.Dashboard(r.Context(), User(r.Context()).ID, p, monitor, models.Window{From: to.Add(-span), To: to})
	if err != nil {
		h.fail(w, err)
		return
	}
	JSON(w, 200, result)
}
