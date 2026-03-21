package handler

import (
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type MonitorService interface {
	GetAll() ([]models.Monitor, error)
	GetByID(id int) (*models.Monitor, error)
	Create(url string, interval, timeout int) (*models.Monitor, error)
	Delete(id int) error
}

type CheckService interface {
	GetByMonitorID(monitorID, limit, offset int) ([]models.Check, error)
	GetLastByMonitorID(monitorID int) (*models.Check, error)
	GetUptimeStats(monitorID int) (*service.UptimeStats, error)
}
