package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/peshk1n/site-monitor/internal/dto"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/repository"
)

type CheckHandler struct {
	checkRepo   *repository.CheckRepository
	monitorRepo *repository.MonitorRepository
}

func NewCheckHandler(
	checkRepo *repository.CheckRepository,
	monitorRepo *repository.MonitorRepository,
) *CheckHandler {
	return &CheckHandler{
		checkRepo:   checkRepo,
		monitorRepo: monitorRepo,
	}
}

// обрабатывает GET /monitors/{id}/checks
func (h *CheckHandler) GetByMonitorID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	_, err = h.monitorRepo.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Monitor not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	checks, err := h.checkRepo.GetByMonitorID(id)
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

// возвращает последнюю проверку для монитора GET /monitors/{id}/checks/last
func (h *CheckHandler) GetLastByMonitorID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid monitor ID", http.StatusBadRequest)
		return
	}

	_, err = h.monitorRepo.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Monitor not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	check, err := h.checkRepo.GetLastCheck(id)
	if err == sql.ErrNoRows {
		http.Error(w, "No checks found", http.StatusNotFound)
		return
	}
	if err != nil {
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
