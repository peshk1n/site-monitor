package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/peshk1n/site-monitor/internal/config"
	"github.com/peshk1n/site-monitor/internal/db"
	"github.com/peshk1n/site-monitor/internal/handler"
	"github.com/peshk1n/site-monitor/internal/repository"
	"github.com/peshk1n/site-monitor/internal/router"
	"github.com/peshk1n/site-monitor/internal/scheduler"
)

func main() {
	cfg := config.Load()
	database := db.Connect(cfg.DBUrl)
	defer database.Close()

	monitorRepo := repository.NewMonitorRepository(database)
	checkRepo := repository.NewCheckRepository(database)
	monitorHandler := handler.NewMonitorHandler(monitorRepo)
	checkHandler := handler.NewCheckHandler(checkRepo, monitorRepo)
	r := router.NewRouter(monitorHandler, checkHandler)
	s := scheduler.NewScheduler(monitorRepo, checkRepo)
	s.Start()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("Завершение работы...")
		s.Stop()
		database.Close()
		os.Exit(0)
	}()
	log.Println("Сервер запущен на порту", cfg.ServerPort)
	if err := http.ListenAndServe(":"+cfg.ServerPort, r); err != nil {
		log.Fatal(err)
	}
}
