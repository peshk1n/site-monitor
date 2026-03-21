package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/peshk1n/site-monitor/internal/dto"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type CheckHandler struct {
	checkService CheckService
}

func NewCheckHandler(checkService CheckService) *CheckHandler {
	return &CheckHandler{
		checkService: checkService,
	}
}

// GET /monitors/{id}/checks
func (h *CheckHandler) GetByMonitorID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	checks, err := h.checkService.GetByMonitorID(id, limit, offset)
	if errors.Is(err, service.ErrMonitorNotFound) {
		http.Error(w, "Monitor not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch checks", http.StatusInternalServerError)
		return
	}

	response := dto.CheckListResponse{
		Checks: make([]dto.CheckResponse, 0, len(checks)),
	}
	for _, c := range checks {
		response.Checks = append(response.Checks, toCheckResponse(c))
	}

	respondJSON(w, http.StatusOK, response)
}

// GET /monitors/{id}/checks/last
func (h *CheckHandler) GetLastByMonitorID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	check, err := h.checkService.GetLastByMonitorID(id)
	if err != nil {
		if errors.Is(err, service.ErrMonitorNotFound) {
			http.Error(w, "Monitor not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrNoChecksFound) {
			http.Error(w, "No checks found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to fetch last check", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toCheckResponse(*check))
}

func toCheckResponse(c models.Check) dto.CheckResponse {
	return dto.CheckResponse{
		ID:         c.ID,
		MonitorID:  c.MonitorID,
		StatusCode: c.StatusCode,
		ResponseMs: c.ResponseMs,
		IsUp:       c.IsUp,
		Error:      c.Error,
		CheckedAt:  c.CheckedAt.Format(time.RFC3339),
	}
}

// GET /monitors/{id}/uptime
func (h *CheckHandler) GetUptimeStats(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	stats, err := h.checkService.GetUptimeStats(id)
	if errors.Is(err, service.ErrMonitorNotFound) {
		http.Error(w, "Monitor not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch uptime stats", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, dto.UptimeStatsResponse{
		Uptime24h: fmt.Sprintf("%.2f%%", stats.Uptime24h),
		Uptime7d:  fmt.Sprintf("%.2f%%", stats.Uptime7d),
		Uptime30d: fmt.Sprintf("%.2f%%", stats.Uptime30d),
	})
}
