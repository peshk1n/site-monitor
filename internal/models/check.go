package models

import "time"

type Check struct {
	ID         int       `db:"id"`
	MonitorID  int       `db:"monitor_id"`
	StatusCode int       `db:"status_code"`
	ResponseMs int       `db:"response_ms"`
	IsUp       bool      `db:"is_up"`
	Error      string    `db:"error"`
	CheckedAt  time.Time `db:"checked_at"`
}
