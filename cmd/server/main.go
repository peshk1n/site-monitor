package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/peshk1n/site-monitor/internal/auth"
	"github.com/peshk1n/site-monitor/internal/config"
	"github.com/peshk1n/site-monitor/internal/db"
	"github.com/peshk1n/site-monitor/internal/handler"
	"github.com/peshk1n/site-monitor/internal/logger"
	"github.com/peshk1n/site-monitor/internal/probe"
	"github.com/peshk1n/site-monitor/internal/repository"
	"github.com/peshk1n/site-monitor/internal/router"
	"github.com/peshk1n/site-monitor/internal/scheduler"
	"github.com/peshk1n/site-monitor/internal/service"
)

func run() error {
	log := logger.New()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	database, err := db.Connect(connectCtx, cfg.DBURL)
	cancel()
	if err != nil {
		return err
	}
	defer database.Close()
	app := service.New(repository.New(database))
	authentication := auth.NewTelegram(cfg.TelegramToken, app, cfg.AuthMaxAge)
	h := handler.New(app, authentication, log)
	r := router.New(h, database.PingContext)
	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	checker := probe.New()
	defer checker.Close()
	worker := scheduler.New(app, checker, log, cfg.Workers, fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()))
	workersDone := make(chan struct{})
	go func() { worker.Run(workerCtx); close(workersDone) }()
	srv := &http.Server{Addr: ":" + cfg.ServerPort, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	serverError := make(chan error, 1)
	go func() { serverError <- srv.ListenAndServe() }()
	log.Info("server started", "port", cfg.ServerPort, "workers", cfg.Workers)
	select {
	case <-ctx.Done():
	case err = <-serverError:
	}
	stopWorkers()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		srv.Close()
		log.Error("HTTP shutdown failed")
	}
	select {
	case <-workersDone:
	case <-shutdownCtx.Done():
		return errors.New("workers did not stop before shutdown deadline")
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		logger.New().Error("server stopped", "error", err)
		os.Exit(1)
	}
}
