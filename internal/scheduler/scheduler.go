package scheduler

import (
	"context"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type Scheduler struct {
	monitorService *service.MonitorService
	checkService   *service.CheckService
	log            *slog.Logger
}

func NewScheduler(
	monitorService *service.MonitorService,
	checkService *service.CheckService,
	log *slog.Logger,
) *Scheduler {
	return &Scheduler{
		monitorService: monitorService,
		checkService:   checkService,
		log:            log,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.log.Info("Scheduler started")

	go func() {
		s.runChecks(ctx)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runChecks(ctx)
			case <-ctx.Done():
				s.log.Info("Scheduler stopped")
				return
			}
		}
	}()
}

func (s *Scheduler) runChecks(ctx context.Context) {
	monitors, err := s.monitorService.GetAll()
	if err != nil {
		s.log.Error("Failed to load monitors:", "error", err)
		return
	}

	if len(monitors) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, monitor := range monitors {
		if !monitor.IsActive {
			continue
		}

		wg.Add(1)
		go func(m models.Monitor) {
			defer wg.Done()
			if err := s.checkService.RunCheck(ctx, m); err != nil {
				log.Println("Failed to check monitor:", "url", m.URL, "error", err)
			}
		}(monitor)
	}

	wg.Wait()
	s.log.Info("All checks completed")
}
