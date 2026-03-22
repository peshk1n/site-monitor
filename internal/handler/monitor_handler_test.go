package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/peshk1n/site-monitor/internal/handler"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type mockMonitorService struct {
	GetAllFn  func() ([]models.Monitor, error)
	GetByIDFn func(id int) (*models.Monitor, error)
	CreateFn  func(url string, interval, timeout int) (*models.Monitor, error)
	UpdateFn  func(id int, interval, timeout *int, isActive *bool) (*models.Monitor, error)
	DeleteFn  func(id int) error
}

func (m *mockMonitorService) GetAll() ([]models.Monitor, error) {
	return m.GetAllFn()
}
func (m *mockMonitorService) GetByID(id int) (*models.Monitor, error) {
	return m.GetByIDFn(id)
}
func (m *mockMonitorService) Create(url string, interval, timeout int) (*models.Monitor, error) {
	return m.CreateFn(url, interval, timeout)
}
func (m *mockMonitorService) Update(id int, interval, timeout *int, isActive *bool) (*models.Monitor, error) {
	return m.UpdateFn(id, interval, timeout, isActive)
}
func (m *mockMonitorService) Delete(id int) error {
	return m.DeleteFn(id)
}

func newTestRouter(h *handler.MonitorHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/monitors", h.GetAll)
	r.Post("/monitors", h.Create)
	r.Get("/monitors/{id}", h.GetByID)
	r.Patch("/monitors/{id}", h.Update)
	r.Delete("/monitors/{id}", h.Delete)
	return r
}

// --- Тесты GetAll ---

func TestMonitorHandler_GetAll_Success(t *testing.T) {
	mock := &mockMonitorService{
		GetAllFn: func() ([]models.Monitor, error) {
			return []models.Monitor{
				{ID: 1, URL: "https://google.com"},
				{ID: 2, URL: "https://github.com"},
			}, nil
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string][]map[string]any
	json.NewDecoder(w.Body).Decode(&response)
	if len(response["monitors"]) != 2 {
		t.Errorf("expected 2 monitors, got %d", len(response["monitors"]))
	}
}

func TestMonitorHandler_GetAll_ServiceError(t *testing.T) {
	mock := &mockMonitorService{
		GetAllFn: func() ([]models.Monitor, error) {
			return nil, errors.New("database error")
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// --- Тесты GetByID ---

func TestMonitorHandler_GetByID_Success(t *testing.T) {
	mock := &mockMonitorService{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return &models.Monitor{ID: id, URL: "https://google.com"}, nil
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMonitorHandler_GetByID_NotFound(t *testing.T) {
	mock := &mockMonitorService{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return nil, service.ErrMonitorNotFound
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestMonitorHandler_GetByID_InvalidID(t *testing.T) {
	mock := &mockMonitorService{}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/monitors/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// --- Тесты Create ---

func TestMonitorHandler_Create_Success(t *testing.T) {
	mock := &mockMonitorService{
		CreateFn: func(url string, interval, timeout int) (*models.Monitor, error) {
			return &models.Monitor{ID: 1, URL: url, Interval: interval, Timeout: timeout}, nil
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	body := `{"url": "https://google.com", "interval": 60}`
	req := httptest.NewRequest(http.MethodPost, "/monitors", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestMonitorHandler_Create_EmptyURL(t *testing.T) {
	mock := &mockMonitorService{
		CreateFn: func(url string, interval, timeout int) (*models.Monitor, error) {
			return nil, service.ErrURLRequired
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	body := `{"url": "", "interval": 60}`
	req := httptest.NewRequest(http.MethodPost, "/monitors", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestMonitorHandler_Create_InvalidJSON(t *testing.T) {
	mock := &mockMonitorService{}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/monitors", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// --- Тесты Update ---

func TestMonitorHandler_Update_Success(t *testing.T) {
	mock := &mockMonitorService{
		UpdateFn: func(id int, interval, timeout *int, isActive *bool) (*models.Monitor, error) {
			return &models.Monitor{ID: id, Interval: 120}, nil
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	body := `{"interval": 120}`
	req := httptest.NewRequest(http.MethodPatch, "/monitors/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMonitorHandler_Update_NotFound(t *testing.T) {
	mock := &mockMonitorService{
		UpdateFn: func(id int, interval, timeout *int, isActive *bool) (*models.Monitor, error) {
			return nil, service.ErrMonitorNotFound
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	body := `{"interval": 120}`
	req := httptest.NewRequest(http.MethodPatch, "/monitors/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestMonitorHandler_Update_InvalidID(t *testing.T) {
	mock := &mockMonitorService{}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	body := `{"interval": 120}`
	req := httptest.NewRequest(http.MethodPatch, "/monitors/abc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestMonitorHandler_Update_InvalidJSON(t *testing.T) {
	mock := &mockMonitorService{}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/monitors/1", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// --- Тесты Delete ---

func TestMonitorHandler_Delete_Success(t *testing.T) {
	mock := &mockMonitorService{
		DeleteFn: func(id int) error {
			return nil
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/monitors/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestMonitorHandler_Delete_NotFound(t *testing.T) {
	mock := &mockMonitorService{
		DeleteFn: func(id int) error {
			return service.ErrMonitorNotFound
		},
	}

	h := handler.NewMonitorHandler(mock)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/monitors/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
