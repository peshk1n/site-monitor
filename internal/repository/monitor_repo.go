package repository

import (
	"github.com/jmoiron/sqlx"
	"github.com/peshk1n/site-monitor/internal/models"
)

type MonitorRepository struct {
	db *sqlx.DB
}

func NewMonitorRepository(db *sqlx.DB) *MonitorRepository {
	return &MonitorRepository{db: db}
}

// возвращает все мониторы из базы
func (r *MonitorRepository) GetAll() ([]models.Monitor, error) {
	var monitors []models.Monitor
	err := r.db.Select(&monitors, "SELECT * FROM monitors")
	return monitors, err
}

// возвращает один монитор по его ID
func (r *MonitorRepository) GetByID(id int) (*models.Monitor, error) {
	var monitor models.Monitor
	err := r.db.Get(&monitor, "SELECT * FROM monitors WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

// добавляет новый монитор в базу
func (r *MonitorRepository) Create(monitor *models.Monitor) error {
	query := `
        INSERT INTO monitors (url, interval, timeout, is_active)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at`

	return r.db.QueryRow(query,
		monitor.URL,
		monitor.Interval,
		monitor.Timeout,
		monitor.IsActive,
	).Scan(&monitor.ID, &monitor.CreatedAt)
}

// удаляет монитор по ID
func (r *MonitorRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM monitors WHERE id = $1", id)
	return err
}
