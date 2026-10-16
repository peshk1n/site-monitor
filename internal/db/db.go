package db

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func Connect(ctx context.Context, url string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", url)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("database connection failed")
	}
	var available bool
	err = db.GetContext(ctx, &available, `SELECT count(*)=3 FROM information_schema.columns WHERE table_schema=current_schema()
 AND ((table_name='users' AND column_name='telegram_user_id') OR (table_name='monitor_runtime' AND column_name='candidate_status') OR (table_name='creation_requests' AND column_name='request_key'))`)
	if err != nil || !available {
		db.Close()
		return nil, errors.New("database schema is not ready; apply migrations through version 013")
	}
	return db, nil
}
