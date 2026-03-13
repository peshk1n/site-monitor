package models

import "time"

type Monitor struct {
	ID        int       `db:"id"`
	URL       string    `db:"url"`
	Interval  int       `db:"interval"`
	Timeout   int       `db:"timeout"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
}
