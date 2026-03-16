package service

import "github.com/peshk1n/site-monitor/internal/models"

type MonitorRepository interface {
	GetAll() ([]models.Monitor, error)
	GetByID(id int) (*models.Monitor, error)
	Create(monitor *models.Monitor) error
	Delete(id int) error
}

type CheckRepository interface {
	Create(check *models.Check) error
	GetByMonitorID(monitorID int) ([]models.Check, error)
	GetLastCheck(monitorID int) (*models.Check, error)
}

type Notifier interface {
	SendAlert(siteURL string, isUp bool, responseMs int)
}
