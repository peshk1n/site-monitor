package service_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type mockCheckRepository struct {
	CreateFn         func(check *models.Check) error
	GetByMonitorIDFn func(monitorID, limit, offset int) ([]models.Check, error)
	GetLastCheckFn   func(monitorID int) (*models.Check, error)
	GetUptimeStatsFn func(monitorID int, since time.Time) (int, int, error)
}

func (m *mockCheckRepository) Create(check *models.Check) error {
	return m.CreateFn(check)
}
func (m *mockCheckRepository) GetByMonitorID(monitorID, limit, offset int) ([]models.Check, error) {
	return m.GetByMonitorIDFn(monitorID, limit, offset)
}
func (m *mockCheckRepository) GetLastCheck(monitorID int) (*models.Check, error) {
	return m.GetLastCheckFn(monitorID)
}
func (m *mockCheckRepository) GetUptimeStats(monitorID int, since time.Time) (int, int, error) {
	return m.GetUptimeStatsFn(monitorID, since)
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
		GetByMonitorIDFn: func(monitorID, limit, offset int) ([]models.Check, error) {
			return expected, nil
		},
	}
	monitorRepo := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return &models.Monitor{ID: id}, nil
		},
	}

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{}, slog.Default())
	checks, err := svc.GetByMonitorID(1, 20, 0)

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

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{}, slog.Default())
	_, err := svc.GetByMonitorID(999, 20, 0)

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

	svc := service.NewCheckService(checkRepo, monitorRepo, notifier, slog.Default())
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

	svc := service.NewCheckService(checkRepo, monitorRepo, notifier, slog.Default())
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

	svc := service.NewCheckService(checkRepo, monitorRepo, notifier, slog.Default())
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

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{}, slog.Default())
	err := svc.RunCheck(context.Background(), models.Monitor{
		ID:      1,
		URL:     "https://google.com",
		Timeout: 10,
	})
	if err == nil {
		t.Error("expected error when save fails")
	}
}

// --- Тесты GetUptimeStats ---

func TestCheckService_GetUptimeStats_Success(t *testing.T) {
	checkRepo := &mockCheckRepository{
		GetUptimeStatsFn: func(monitorID int, since time.Time) (int, int, error) {
			return 100, 99, nil
		},
	}
	monitorRepo := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return &models.Monitor{ID: id}, nil
		},
	}

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{}, slog.Default())
	stats, err := svc.GetUptimeStats(1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.Uptime24h != 99.0 {
		t.Errorf("expected uptime 99.0, got %.2f", stats.Uptime24h)
	}
}

func TestCheckService_GetUptimeStats_MonitorNotFound(t *testing.T) {
	checkRepo := &mockCheckRepository{}
	monitorRepo := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return nil, sql.ErrNoRows
		},
	}

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{}, slog.Default())
	_, err := svc.GetUptimeStats(999)

	if !errors.Is(err, service.ErrMonitorNotFound) {
		t.Errorf("expected ErrMonitorNotFound, got %v", err)
	}
}

func TestCheckService_GetUptimeStats_NoChecks(t *testing.T) {
	checkRepo := &mockCheckRepository{
		GetUptimeStatsFn: func(monitorID int, since time.Time) (int, int, error) {
			return 0, 0, nil
		},
	}
	monitorRepo := &mockMonitorRepository{
		GetByIDFn: func(id int) (*models.Monitor, error) {
			return &models.Monitor{ID: id}, nil
		},
	}

	svc := service.NewCheckService(checkRepo, monitorRepo, &mockNotifier{}, slog.Default())
	stats, err := svc.GetUptimeStats(1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.Uptime24h != 0 {
		t.Errorf("expected uptime 0, got %.2f", stats.Uptime24h)
	}
}
