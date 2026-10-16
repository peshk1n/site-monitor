package service

import (
	"context"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

type Store interface {
	Within(context.Context, func(Repository) error) error
	View(context.Context, func(Repository) error) error
	Claim(context.Context, string) (*models.Job, error)
}

type Repository interface {
	Creation(context.Context, int64, string, string, string) ([]byte, error)
	SaveCreation(context.Context, int64, string, []byte) error
	User(context.Context, int64, string) (models.User, error)
	Projects(context.Context, int64, bool, models.Page) ([]models.Project, error)
	Project(context.Context, int64, string) (models.Project, error)
	Role(context.Context, int64, int64, bool) (string, error)
	CreateProject(context.Context, int64, string) (models.Project, error)
	SaveProject(context.Context, models.Project) error
	Monitors(context.Context, int64, models.MonitorFilter) ([]models.MonitorView, error)
	Monitor(context.Context, int64, int64, bool) (models.Monitor, error)
	MonitorView(context.Context, int64, int64) (models.MonitorView, error)
	CreateMonitor(context.Context, *models.Monitor) error
	SaveMonitor(context.Context, *models.Monitor) error
	Runtime(context.Context, int64, bool) (models.Runtime, error)
	SaveRuntime(context.Context, models.Runtime) error
	OpenIncident(context.Context, int64) (*models.Incident, error)
	SaveIncident(context.Context, *models.Incident) error
	Event(context.Context, models.Event) error
	CancelDeliveries(context.Context, int64) error
	Audit(context.Context, int64, int64, string, string, int64) error
	Results(context.Context, int64, models.Page) ([]models.Result, error)
	InsertResult(context.Context, *models.Result) (bool, error)
	Incidents(context.Context, int64, *int64, *models.Window, bool, models.Page) ([]models.Incident, error)
	Summary(context.Context, int64) (map[string]int64, error)
	Chart(context.Context, int64, models.Window, time.Duration) ([]models.ChartPoint, error)
}
