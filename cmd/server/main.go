package main

import (
	"log"

	"github.com/peshk1n/site-monitor/internal/config"
	"github.com/peshk1n/site-monitor/internal/db"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/repository"
)

func main() {
	cfg := config.Load()

	database := db.Connect(cfg.DBUrl)
	defer database.Close()
	monitorRepo := repository.NewMonitorRepository(database)
	testMonitor := &models.Monitor{
		URL:      "https://google.com",
		Interval: 60,
		Timeout:  10,
		IsActive: true,
	}

	err := monitorRepo.Create(testMonitor)
	if err != nil {
		log.Fatal("Ошибка при создании монитора: ", err)
	}
	log.Println("Монитор создан, ID:", testMonitor.ID)

	monitors, err := monitorRepo.GetAll()
	if err != nil {
		log.Fatal("Ошибка при получении мониторов: ", err)
	}
	log.Println("Мониторов в базе:", len(monitors))
	for _, m := range monitors {
		log.Println("-", m.URL)
	}
}
