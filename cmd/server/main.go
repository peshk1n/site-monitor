package main

import (
	"log"
	"net/http"

	"github.com/peshk1n/site-monitor/internal/config"
	"github.com/peshk1n/site-monitor/internal/db"
	"github.com/peshk1n/site-monitor/internal/handler"
	"github.com/peshk1n/site-monitor/internal/repository"
	"github.com/peshk1n/site-monitor/internal/router"
)

func main() {
	cfg := config.Load()
	database := db.Connect(cfg.DBUrl)
	defer database.Close()

	monitorRepo := repository.NewMonitorRepository(database)
	monitorHandler := handler.NewMonitorHandler(monitorRepo)
	router := router.NewRouter(monitorHandler)

	log.Println("Сервер запущен на порту", cfg.ServerPort)
	if err := http.ListenAndServe(":"+cfg.ServerPort, router); err != nil {
		log.Fatal(err)
	}
}
