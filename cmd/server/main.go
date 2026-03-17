package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/peshk1n/site-monitor/internal/bot"
	"github.com/peshk1n/site-monitor/internal/config"
	"github.com/peshk1n/site-monitor/internal/db"
	"github.com/peshk1n/site-monitor/internal/handler"
	"github.com/peshk1n/site-monitor/internal/repository"
	"github.com/peshk1n/site-monitor/internal/router"
	"github.com/peshk1n/site-monitor/internal/scheduler"
	"github.com/peshk1n/site-monitor/internal/service"
)

func main() {
	cfg := config.Load()

	database := db.Connect(cfg.DBUrl)
	defer database.Close()

	monitorRepo := repository.NewMonitorRepository(database)
	checkRepo := repository.NewCheckRepository(database)

	telegramBot, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal("Failed to create telegram bot:", err)
	}

	notifier := bot.NewTelegramNotifier(telegramBot, cfg.TelegramChatID)
	monitorService := service.NewMonitorService(monitorRepo)
	checkService := service.NewCheckService(checkRepo, monitorRepo, notifier)
	tgBot := bot.NewBot(telegramBot, cfg.TelegramChatID, monitorService, checkService)
	tgBot.Start()
	s := scheduler.NewScheduler(monitorService, checkService)
	s.Start()
	monitorHandler := handler.NewMonitorHandler(monitorService)
	checkHandler := handler.NewCheckHandler(checkService)
	r := router.NewRouter(monitorHandler, checkHandler)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Завершение работы...")
		s.Stop()
		tgBot.Stop()
		database.Close()
		os.Exit(0)
	}()

	log.Println("Сервер запущен на порту", cfg.ServerPort)
	if err := http.ListenAndServe(":"+cfg.ServerPort, r); err != nil {
		log.Fatal(err)
	}
}
