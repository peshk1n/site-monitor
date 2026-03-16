package service

import (
	"errors"

	"database/sql"

	"github.com/peshk1n/site-monitor/internal/models"
)

type MonitorService struct {
	monitorRepo MonitorRepository
}

func NewMonitorService(monitorRepo MonitorRepository) *MonitorService {
	return &MonitorService{
		monitorRepo: monitorRepo,
	}
}

func (s *MonitorService) GetAll() ([]models.Monitor, error) {
	return s.monitorRepo.GetAll()
}

func (s *MonitorService) GetByID(id int) (*models.Monitor, error) {
	monitor, err := s.monitorRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMonitorNotFound
		}
		return nil, err
	}
	return monitor, nil
}

func (s *MonitorService) Create(url string, interval, timeout int) (*models.Monitor, error) {
	if url == "" {
		return nil, ErrURLRequired
	}

	if interval == 0 {
		interval = 60
	}
	if timeout == 0 {
		timeout = 10
	}

	monitor := &models.Monitor{
		URL:      url,
		Interval: interval,
		Timeout:  timeout,
		IsActive: true,
	}

	if err := s.monitorRepo.Create(monitor); err != nil {
		return nil, err
	}
	return monitor, nil
}

func (s *MonitorService) Delete(id int) error {
	err := s.monitorRepo.Delete(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMonitorNotFound
		}
		return err
	}
	return nil
}
