package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/peshk1n/site-monitor/internal/dto"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/repository"
)

type MonitorHandler struct {
	monitorRepo *repository.MonitorRepository
}

func NewMonitorHandler(monitorRepo *repository.MonitorRepository) *MonitorHandler {
	return &MonitorHandler{monitorRepo: monitorRepo}
}

// обрабатывает GET /monitors
func (h *MonitorHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.monitorRepo.GetAll()
	if err != nil {
		http.Error(w, "Failed to fetch monitors", http.StatusInternalServerError)
		return
	}

	response := dto.MonitorListResponse{
		Monitors: make([]dto.MonitorResponse, 0, len(monitors)),
	}
	for _, m := range monitors {
		response.Monitors = append(response.Monitors, toMonitorResponse(m))
	}

	respondJSON(w, http.StatusOK, response)
}

// обрабатывает GET /monitors/{id}
func (h *MonitorHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	monitor, err := h.monitorRepo.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Monitor not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toMonitorResponse(*monitor))
}

// обрабатывает POST /monitors
func (h *MonitorHandler) Create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req dto.CreateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	monitor := &models.Monitor{
		URL:      req.URL,
		Interval: req.Interval,
		Timeout:  req.Timeout,
		IsActive: true,
	}

	if monitor.Interval == 0 {
		monitor.Interval = 60
	}
	if monitor.Timeout == 0 {
		monitor.Timeout = 10
	}

	if err := h.monitorRepo.Create(monitor); err != nil {
		http.Error(w, "Failed to create monitor", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, toMonitorResponse(*monitor))
}

// обрабатывает DELETE /monitors/{id}
func (h *MonitorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	if err := h.monitorRepo.Delete(id); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Monitor not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to delete monitor", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toMonitorResponse(m models.Monitor) dto.MonitorResponse {
	return dto.MonitorResponse{
		ID:        m.ID,
		URL:       m.URL,
		Interval:  m.Interval,
		Timeout:   m.Timeout,
		IsActive:  m.IsActive,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
}

// превращает структуру в JSON и отправляет клиенту
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
