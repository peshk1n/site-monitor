package service_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type mockMonitorRepository struct {
	GetAllFn  func() ([]models.Monitor, error)
	GetByIDFn func(id int) (*models.Monitor, error)
	CreateFn  func(monitor *models.Monitor) error
	DeleteFn  func(id int) error
}

func (m *mockMonitorRepository) GetAll() ([]models.Monitor, error) {
	return m.GetAllFn()
}
func (m *mockMonitorRepository) GetByID(id int) (*models.Monitor, error) {
	return m.GetByIDFn(id)
}
func (m *mockMonitorRepository) Create(monitor *models.Monitor) error {
	return m.CreateFn(monitor)
}
func (m *mockMonitorRepository) Delete(id int) error {
	return m.DeleteFn(id)
}

// --- Тесты Create ---

func TestMonitorService_Create_Success(t *testing.T) {
	mock := &mockMonitorRepository{
		CreateFn: func(monitor *models.Monitor) error {
			monitor.ID = 1
			return nil
		},
	}

	svc := service.NewMonitorService(mock)
	monitor, err := svc.Create("https://google.com", 60, 10)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if monitor.ID != 1 {
		t.Errorf("expected ID 1, got %d", monitor.ID)
	}
	if monitor.URL != "https://google.com" {
		t.Errorf("expected URL https://google.com, got %s", monitor.URL)
	}
	if !monitor.IsActive {
		t.Error("expected monitor to be active")
	}
}

func TestMonitorService_Create_EmptyURL(t *testing.T) {
	mock := &mockMonitorRepository{}
	svc := service.NewMonitorService(mock)

	_, err := svc.Create("", 60, 10)

	if !errors.Is(err, service.ErrURLRequired) {
		t.Errorf("expected ErrURLRequired, got %v", err)
	}
}

func TestMonitorService_Create_DefaultValues(t *testing.T) {
	mock := &mockMonitorRepository{
		CreateFn: func(monitor *models.Monitor) error {
			return nil
		},
	}

	svc := service.NewMonitorService(mock)
	monitor, err := svc.Create("https://google.com", 0, 0)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if monitor.Interval != 60 {
		t.Errorf("expected interval 60, got %d", monitor.Interval)
	}
	if monitor.Timeout != 10 {
		t.Errorf("expected timeout 10, got %d", monitor.Timeout)
	}
}

// --- Тесты GetByID ---

func TestMonitorService_GetByID_Success(t *testing.T) {
	expected := &models.Monitor{ID: 1, URL: "https://google.com"}

	mock := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return expected, nil
		},
	}

	svc := service.NewMonitorService(mock)
	monitor, err := svc.GetByID(1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if monitor.ID != expected.ID {
		t.Errorf("expected ID %d, got %d", expected.ID, monitor.ID)
	}
}

func TestMonitorService_GetByID_NotFound(t *testing.T) {
	mock := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return nil, sql.ErrNoRows
		},
	}

	svc := service.NewMonitorService(mock)
	_, err := svc.GetByID(999)

	if !errors.Is(err, service.ErrMonitorNotFound) {
		t.Errorf("expected ErrMonitorNotFound, got %v", err)
	}
}

// --- Тесты Delete ---

func TestMonitorService_Delete_Success(t *testing.T) {
	mock := &mockMonitorRepository{
		DeleteFn: func(id int) error {
			return nil
		},
	}

	svc := service.NewMonitorService(mock)
	err := svc.Delete(1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMonitorService_Delete_NotFound(t *testing.T) {
	mock := &mockMonitorRepository{
		DeleteFn: func(id int) error {
			return sql.ErrNoRows
		},
	}

	svc := service.NewMonitorService(mock)
	err := svc.Delete(999)

	if !errors.Is(err, service.ErrMonitorNotFound) {
		t.Errorf("expected ErrMonitorNotFound, got %v", err)
	}
}

// --- Тесты GetAll ---

func TestMonitorService_GetAll_Empty(t *testing.T) {
	mock := &mockMonitorRepository{
		GetAllFn: func() ([]models.Monitor, error) {
			return []models.Monitor{}, nil
		},
	}

	svc := service.NewMonitorService(mock)
	monitors, err := svc.GetAll()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(monitors) != 0 {
		t.Errorf("expected 0 monitors, got %d", len(monitors))
	}
}

func TestMonitorService_GetAll_ReturnsMonitors(t *testing.T) {
	expected := []models.Monitor{
		{ID: 1, URL: "https://google.com"},
		{ID: 2, URL: "https://github.com"},
	}

	mock := &mockMonitorRepository{
		GetAllFn: func() ([]models.Monitor, error) {
			return expected, nil
		},
	}

	svc := service.NewMonitorService(mock)
	monitors, err := svc.GetAll()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(monitors) != 2 {
		t.Errorf("expected 2 monitors, got %d", len(monitors))
	}
}
