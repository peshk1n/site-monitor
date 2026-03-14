package service

import (
	"database/sql"
	"errors"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/repository"
)

type CheckService struct {
	checkRepo   *repository.CheckRepository
	monitorRepo *repository.MonitorRepository
}

func NewCheckService(
	checkRepo *repository.CheckRepository,
	monitorRepo *repository.MonitorRepository,
) *CheckService {
	return &CheckService{
		checkRepo:   checkRepo,
		monitorRepo: monitorRepo,
	}
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
