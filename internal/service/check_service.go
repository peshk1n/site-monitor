package service

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/repository"
)

type Notifier interface {
	SendAlert(siteURL string, isUp bool, responseMs int)
}

type CheckService struct {
	checkRepo   *repository.CheckRepository
	monitorRepo *repository.MonitorRepository
	notifier    Notifier
}

func NewCheckService(
	checkRepo *repository.CheckRepository,
	monitorRepo *repository.MonitorRepository,
	notifier Notifier,
) *CheckService {
	return &CheckService{
		checkRepo:   checkRepo,
		monitorRepo: monitorRepo,
		notifier:    notifier,
	}
}

func (s *CheckService) RunCheck(monitor models.Monitor) error {
	client := &http.Client{
		Timeout: time.Duration(monitor.Timeout) * time.Second,
	}

	start := time.Now()
	resp, err := client.Get(monitor.URL)
	responseMs := int(time.Since(start).Milliseconds())
	check := &models.Check{
		MonitorID:  monitor.ID,
		ResponseMs: responseMs,
	}

	if err != nil {
		check.IsUp = false
		check.Error = err.Error()
		log.Printf("%s is DOWN: %s\n", monitor.URL, err.Error())
	} else {
		defer resp.Body.Close()
		check.StatusCode = resp.StatusCode
		check.IsUp = resp.StatusCode < 400
		if !check.IsUp {
			check.Error = "HTTP " + http.StatusText(resp.StatusCode)
			log.Printf("%s returned %d\n", monitor.URL, resp.StatusCode)
		} else {
			log.Printf("%s is UP (%d ms)\n", monitor.URL, responseMs)
		}
	}

	lastCheck, lastErr := s.GetLastByMonitorID(monitor.ID)
	if err := s.Create(check); err != nil {
		return err
	}

	if errors.Is(lastErr, sql.ErrNoRows) || lastCheck.IsUp != check.IsUp {
		s.notifier.SendAlert(monitor.URL, check.IsUp, responseMs)
	}

	return nil
}

func (s *CheckService) GetByMonitorID(monitorID int) ([]models.Check, error) {
	_, err := s.monitorRepo.GetByID(monitorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMonitorNotFound
		}
		return nil, err
	}

	return s.checkRepo.GetByMonitorID(monitorID)
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
