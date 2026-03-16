package handler

import "github.com/peshk1n/site-monitor/internal/models"

type MonitorService interface {
	GetAll() ([]models.Monitor, error)
	GetByID(id int) (*models.Monitor, error)
	Create(url string, interval, timeout int) (*models.Monitor, error)
	Delete(id int) error
}

type CheckService interface {
	GetByMonitorID(monitorID int) ([]models.Check, error)
	GetLastByMonitorID(monitorID int) (*models.Check, error)
}
