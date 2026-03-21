package repository

import (
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/peshk1n/site-monitor/internal/models"
)

type CheckRepository struct {
	db *sqlx.DB
}

func NewCheckRepository(db *sqlx.DB) *CheckRepository {
	return &CheckRepository{db: db}
}

// сохраняет результат одной проверки
func (r *CheckRepository) Create(check *models.Check) error {
	query := `
        INSERT INTO checks (monitor_id, status_code, response_ms, is_up, error)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, checked_at`

	return r.db.QueryRow(query,
		check.MonitorID,
		check.StatusCode,
		check.ResponseMs,
		check.IsUp,
		check.Error,
	).Scan(&check.ID, &check.CheckedAt)
}

// возвращает историю проверок для конкретного монитора
func (r *CheckRepository) GetByMonitorID(monitorID, limit, offset int) ([]models.Check, error) {
	var checks []models.Check
	query := `
        SELECT * FROM checks
        WHERE monitor_id = $1
        ORDER BY checked_at DESC
        LIMIT $2 OFFSET $3`

	err := r.db.Select(&checks, query, monitorID, limit, offset)
	return checks, err
}

// возвращает последнюю проверку для монитора
func (r *CheckRepository) GetLastCheck(monitorID int) (*models.Check, error) {
	var check models.Check
	query := `
        SELECT * FROM checks
        WHERE monitor_id = $1
        ORDER BY checked_at DESC
        LIMIT 1`

	err := r.db.Get(&check, query, monitorID)
	if err != nil {
		return nil, err
	}
	return &check, nil
}

// возвращает количество успешных и общее количество проверок за период
func (r *CheckRepository) GetUptimeStats(monitorID int, since time.Time) (total int, successful int, err error) {
	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN is_up THEN 1 ELSE 0 END) as successful
		FROM checks
		WHERE monitor_id = $1 AND checked_at >= $2`

	row := r.db.QueryRow(query, monitorID, since)
	err = row.Scan(&total, &successful)
	return total, successful, err
}
