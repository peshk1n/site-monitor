package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/peshk1n/site-monitor/internal/dto"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type MonitorHandler struct {
	monitorService MonitorService
}

func NewMonitorHandler(monitorService MonitorService) *MonitorHandler {
	return &MonitorHandler{
		monitorService: monitorService,
	}
}

// GET /monitors
func (h *MonitorHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.monitorService.GetAll()
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

// GET /monitors/{id}
func (h *MonitorHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	monitor, err := h.monitorService.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrMonitorNotFound) {
			http.Error(w, "Monitor not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toMonitorResponse(*monitor))
}

// POST /monitors
func (h *MonitorHandler) Create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req dto.CreateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	monitor, err := h.monitorService.Create(
		req.URL,
		req.Interval,
		req.Timeout,
	)
	if err != nil {
		if errors.Is(err, service.ErrURLRequired) {
			http.Error(w, "URL is required", http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to create monitor", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, toMonitorResponse(*monitor))
}

// DELETE /monitors/{id}
func (h *MonitorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	err = h.monitorService.Delete(id)
	if err != nil {
		if errors.Is(err, service.ErrMonitorNotFound) {
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

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
