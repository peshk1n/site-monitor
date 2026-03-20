package db

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func Connect(databaseURL string, log *slog.Logger) *sqlx.DB {
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		log.Error("Error connecting to database", "error", err)
	}

	log.Info("Database connected")
	return db
}
