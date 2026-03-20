package main

import (
	"context"
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

	ctx, cancel := context.WithCancel(context.Background())

	s := scheduler.NewScheduler(monitorService, checkService)
	s.Start(ctx)

	monitorHandler := handler.NewMonitorHandler(monitorService)
	checkHandler := handler.NewCheckHandler(checkService)
	r := router.NewRouter(monitorHandler, checkHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		cancel()
		tgBot.Stop()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Println("HTTP server shutdown error:", err)
		}

		database.Close()
	}()

	log.Println("Server started on port", cfg.ServerPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
