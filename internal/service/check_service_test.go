package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type mockCheckRepository struct {
	CreateFn         func(check *models.Check) error
	GetByMonitorIDFn func(monitorID int) ([]models.Check, error)
	GetLastCheckFn   func(monitorID int) (*models.Check, error)
}

func (m *mockCheckRepository) Create(check *models.Check) error {
	return m.CreateFn(check)
}
func (m *mockCheckRepository) GetByMonitorID(monitorID int) ([]models.Check, error) {
	return m.GetByMonitorIDFn(monitorID)
}
func (m *mockCheckRepository) GetLastCheck(monitorID int) (*models.Check, error) {
	return m.GetLastCheckFn(monitorID)
}

type mockNotifier struct {
	SendAlertFn func(siteURL string, isUp bool, responseMs int)

	SendAlertCalled bool
	LastSiteURL     string
	LastIsUp        bool
}

func (m *mockNotifier) SendAlert(siteURL string, isUp bool, responseMs int) {
	m.SendAlertCalled = true
	m.LastSiteURL = siteURL
	m.LastIsUp = isUp
	if m.SendAlertFn != nil {
		m.SendAlertFn(siteURL, isUp, responseMs)
	}
}

// --- Тесты GetByMonitorID ---

func TestCheckService_GetByMonitorID_Success(t *testing.T) {
	expected := []models.Check{
		{ID: 1, MonitorID: 1, IsUp: true},
		{ID: 2, MonitorID: 1, IsUp: false},
	}

	checkRepo := &mockCheckRepository{
		GetByMonitorIDFn: func(monitorID int) ([]models.Check, error) {
			return expected, nil
		},
	}
	monitorRepo := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return &models.Monitor{ID: id}, nil
		},
	}

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{})
	checks, err := svc.GetByMonitorID(1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(checks))
	}
}

func TestCheckService_GetByMonitorID_MonitorNotFound(t *testing.T) {
	checkRepo := &mockCheckRepository{}
	monitorRepo := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return nil, sql.ErrNoRows
		},
	}

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{})
	_, err := svc.GetByMonitorID(999)

	if !errors.Is(err, service.ErrMonitorNotFound) {
		t.Errorf("expected ErrMonitorNotFound, got %v", err)
	}
}

// --- Тесты RunCheck ---

func TestCheckService_RunCheck_SiteUp_FirstCheck(t *testing.T) {
	notifier := &mockNotifier{}

	checkRepo := &mockCheckRepository{
		GetLastCheckFn: func(monitorID int) (*models.Check, error) {
			return nil, sql.ErrNoRows
		},
		CreateFn: func(check *models.Check) error {
			return nil
		},
	}
	monitorRepo := &mockMonitorRepository{}

	svc := service.NewCheckService(checkRepo, monitorRepo, notifier)
	err := svc.RunCheck(context.Background(), models.Monitor{
		ID:      1,
		URL:     "https://google.com",
		Timeout: 10,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !notifier.SendAlertCalled {
		t.Error("expected SendAlert to be called on first check")
	}
}

func TestCheckService_RunCheck_StatusUnchanged_NoAlert(t *testing.T) {
	notifier := &mockNotifier{}

	checkRepo := &mockCheckRepository{
		GetLastCheckFn: func(monitorID int) (*models.Check, error) {
			return &models.Check{IsUp: true}, nil
		},
		CreateFn: func(check *models.Check) error {
			return nil
		},
	}
	monitorRepo := &mockMonitorRepository{}

	svc := service.NewCheckService(checkRepo, monitorRepo, notifier)
	err := svc.RunCheck(context.Background(), models.Monitor{
		ID:      1,
		URL:     "https://google.com",
		Timeout: 10,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if notifier.SendAlertCalled {
		t.Error("expected SendAlert NOT to be called when status unchanged")
	}
}

func TestCheckService_RunCheck_StatusChanged_AlertSent(t *testing.T) {
	notifier := &mockNotifier{}

	checkRepo := &mockCheckRepository{
		GetLastCheckFn: func(monitorID int) (*models.Check, error) {
			return &models.Check{IsUp: true}, nil
		},
		CreateFn: func(check *models.Check) error {
			return nil
		},
	}
	monitorRepo := &mockMonitorRepository{}

	svc := service.NewCheckService(checkRepo, monitorRepo, notifier)
	err := svc.RunCheck(context.Background(), models.Monitor{
		ID:      1,
		URL:     "https://thiswebsitedoesnotexist123.com",
		Timeout: 5,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !notifier.SendAlertCalled {
		t.Error("expected SendAlert to be called when status changed")
	}
	if notifier.LastIsUp {
		t.Error("expected LastIsUp to be false")
	}
}

func TestCheckService_RunCheck_SaveFailed(t *testing.T) {
	checkRepo := &mockCheckRepository{
		GetLastCheckFn: func(monitorID int) (*models.Check, error) {
			return nil, sql.ErrNoRows
		},
		CreateFn: func(check *models.Check) error {
			return errors.New("database error")
		},
	}
	monitorRepo := &mockMonitorRepository{}

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{})
	err := svc.RunCheck(context.Background(), models.Monitor{
		ID:      1,
		URL:     "https://google.com",
		Timeout: 10,
	})
	if err == nil {
		t.Error("expected error when save fails")
	}
}
