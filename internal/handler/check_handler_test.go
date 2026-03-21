package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/peshk1n/site-monitor/internal/handler"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type mockCheckService struct {
	GetByMonitorIDFn     func(monitorID, limit, offset int) ([]models.Check, error)
	GetLastByMonitorIDFn func(monitorID int) (*models.Check, error)
	GetUptimeStatsFn     func(monitorID int) (*service.UptimeStats, error)
}

func (m *mockCheckService) GetByMonitorID(monitorID, limit, offset int) ([]models.Check, error) {
	return m.GetByMonitorIDFn(monitorID, limit, offset)
}
func (m *mockCheckService) GetLastByMonitorID(monitorID int) (*models.Check, error) {
	return m.GetLastByMonitorIDFn(monitorID)
}
func (m *mockCheckService) GetUptimeStats(monitorID int) (*service.UptimeStats, error) {
	return m.GetUptimeStatsFn(monitorID)
}

func newTestCheckRouter(h *handler.CheckHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/monitors/{id}/checks", h.GetByMonitorID)
	r.Get("/monitors/{id}/checks/last", h.GetLastByMonitorID)
	r.Get("/monitors/{id}/uptime", h.GetUptimeStats)
	return r
}

// --- Тесты GetByMonitorID ---

func TestCheckHandler_GetByMonitorID_Success(t *testing.T) {
	mock := &mockCheckService{
		GetByMonitorIDFn: func(monitorID, limit, offset int) ([]models.Check, error) {
			return []models.Check{
				{ID: 1, MonitorID: monitorID, IsUp: true},
				{ID: 2, MonitorID: monitorID, IsUp: false},
			}, nil
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/1/checks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string][]map[string]any
	json.NewDecoder(w.Body).Decode(&response)
	if len(response["checks"]) != 2 {
		t.Errorf("expected 2 checks, got %d", len(response["checks"]))
	}
}

func TestCheckHandler_GetByMonitorID_WithPagination(t *testing.T) {
	mock := &mockCheckService{
		GetByMonitorIDFn: func(monitorID, limit, offset int) ([]models.Check, error) {
			if limit != 10 || offset != 20 {
				return nil, errors.New("unexpected limit or offset")
			}
			return []models.Check{}, nil
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/1/checks?limit=10&offset=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCheckHandler_GetByMonitorID_MonitorNotFound(t *testing.T) {
	mock := &mockCheckService{
		GetByMonitorIDFn: func(monitorID, limit, offset int) ([]models.Check, error) {
			return nil, service.ErrMonitorNotFound
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/999/checks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCheckHandler_GetByMonitorID_InvalidID(t *testing.T) {
	mock := &mockCheckService{}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/abc/checks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCheckHandler_GetByMonitorID_ServiceError(t *testing.T) {
	mock := &mockCheckService{
		GetByMonitorIDFn: func(monitorID, limit, offset int) ([]models.Check, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/1/checks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// --- Тесты GetLastByMonitorID ---

func TestCheckHandler_GetLastByMonitorID_Success(t *testing.T) {
	mock := &mockCheckService{
		GetLastByMonitorIDFn: func(monitorID int) (*models.Check, error) {
			return &models.Check{ID: 1, MonitorID: monitorID, IsUp: true}, nil
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/1/checks/last", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCheckHandler_GetLastByMonitorID_MonitorNotFound(t *testing.T) {
	mock := &mockCheckService{
		GetLastByMonitorIDFn: func(monitorID int) (*models.Check, error) {
			return nil, service.ErrMonitorNotFound
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/999/checks/last", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCheckHandler_GetLastByMonitorID_NoChecks(t *testing.T) {
	mock := &mockCheckService{
		GetLastByMonitorIDFn: func(monitorID int) (*models.Check, error) {
			return nil, service.ErrNoChecksFound
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/1/checks/last", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// --- Тесты GetUptimeStats ---

func TestCheckHandler_GetUptimeStats_Success(t *testing.T) {
	mock := &mockCheckService{
		GetUptimeStatsFn: func(monitorID int) (*service.UptimeStats, error) {
			return &service.UptimeStats{
				Uptime24h: 99.8,
				Uptime7d:  98.5,
				Uptime30d: 97.2,
			}, nil
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/1/uptime", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCheckHandler_GetUptimeStats_MonitorNotFound(t *testing.T) {
	mock := &mockCheckService{
		GetUptimeStatsFn: func(monitorID int) (*service.UptimeStats, error) {
			return nil, service.ErrMonitorNotFound
		},
	}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/999/uptime", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCheckHandler_GetUptimeStats_InvalidID(t *testing.T) {
	mock := &mockCheckService{}

	h := handler.NewCheckHandler(mock)
	r := newTestCheckRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/abc/uptime", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
