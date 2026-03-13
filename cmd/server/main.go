package main

import (
	"log"

	"github.com/peshk1n/site-monitor/internal/config"
	"github.com/peshk1n/site-monitor/internal/db"
)

func main() {
	cfg := config.Load()

	database := db.Connect(cfg.DBUrl)
	defer database.Close()

	log.Println("Server started on port", cfg.ServerPort)
}
