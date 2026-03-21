package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

type UptimeStats struct {
	Uptime24h float64
	Uptime7d  float64
	Uptime30d float64
}

const (
	recheckCount    = 2
	recheckInterval = 10 * time.Second
)

type CheckService struct {
	checkRepo   CheckRepository
	monitorRepo MonitorRepository
	notifier    Notifier
	log         *slog.Logger
}

func NewCheckService(
	checkRepo CheckRepository,
	monitorRepo MonitorRepository,
	notifier Notifier,
	log *slog.Logger,
) *CheckService {
	return &CheckService{
		checkRepo:   checkRepo,
		monitorRepo: monitorRepo,
		notifier:    notifier,
		log:         log,
	}
}

func (s *CheckService) RunCheck(ctx context.Context, monitor models.Monitor) error {
	check := s.doCheck(ctx, monitor)

	if !check.IsUp {
		for i := 0; i < recheckCount; i++ {
			s.log.Info("Rechecking site",
				"url", monitor.URL,
				"attempt", i+1,
				"of", recheckCount,
			)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(recheckInterval):
			}

			recheck := s.doCheck(ctx, monitor)
			if recheck.IsUp {
				s.log.Info("Site recovered on recheck", "url", monitor.URL)
				check = recheck
				break
			}
		}
	}

	lastCheck, lastErr := s.checkRepo.GetLastCheck(monitor.ID)

	if err := s.checkRepo.Create(check); err != nil {
		return err
	}

	if errors.Is(lastErr, sql.ErrNoRows) || lastCheck.IsUp != check.IsUp {
		s.notifier.SendAlert(monitor.URL, check.IsUp, check.ResponseMs)
	}

	return nil
}

func (s *CheckService) doCheck(ctx context.Context, monitor models.Monitor) *models.Check {
	client := &http.Client{
		Timeout: time.Duration(monitor.Timeout) * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, monitor.URL, nil)
	if err != nil {
		return &models.Check{
			MonitorID: monitor.ID,
			IsUp:      false,
			Error:     err.Error(),
		}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SiteMonitor/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	start := time.Now()
	resp, err := client.Do(req)
	responseMs := int(time.Since(start).Milliseconds())
	check := &models.Check{
		MonitorID:  monitor.ID,
		ResponseMs: responseMs,
	}

	if err != nil {
		if ctx.Err() != nil {
			return check
		}
		check.IsUp = false
		check.Error = err.Error()
		s.log.Warn("Site is DOWN", "url", monitor.URL, "error", err.Error())
	} else {
		defer resp.Body.Close()
		check.StatusCode = resp.StatusCode
		check.IsUp = resp.StatusCode < 400
		if !check.IsUp {
			check.Error = "HTTP " + http.StatusText(resp.StatusCode)
			s.log.Warn("Site returned error status",
				"url", monitor.URL,
				"status_code", resp.StatusCode,
			)
		} else {
			s.log.Info("Site is UP", "url", monitor.URL, "duration_ms", responseMs)
		}
	}

	return check
}

func (s *CheckService) GetByMonitorID(monitorID, limit, offset int) ([]models.Check, error) {
	_, err := s.monitorRepo.GetByID(monitorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMonitorNotFound
		}
		return nil, err
	}

	return s.checkRepo.GetByMonitorID(monitorID, limit, offset)
}

func (s *CheckService) GetLastByMonitorID(monitorID int) (*models.Check, error) {
	_, err := s.monitorRepo.GetByID(monitorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMonitorNotFound
		}
		return nil, err
	}

	check, err := s.checkRepo.GetLastCheck(monitorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoChecksFound
		}
		return nil, err
	}

	return check, nil
}

func (s *CheckService) Create(check *models.Check) error {
	return s.checkRepo.Create(check)
}

func (s *CheckService) GetUptimeStats(monitorID int) (*UptimeStats, error) {
	_, err := s.monitorRepo.GetByID(monitorID)
	if err != nil {
		return nil, ErrMonitorNotFound
	}

	now := time.Now()

	calc := func(since time.Time) float64 {
		total, successful, err := s.checkRepo.GetUptimeStats(monitorID, since)
		if err != nil || total == 0 {
			return 0
		}
		return float64(successful) / float64(total) * 100
	}

	return &UptimeStats{
		Uptime24h: calc(now.Add(-24 * time.Hour)),
		Uptime7d:  calc(now.Add(-7 * 24 * time.Hour)),
		Uptime30d: calc(now.Add(-30 * 24 * time.Hour)),
	}, nil
}
