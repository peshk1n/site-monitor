package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type Scheduler struct {
	monitorService *service.MonitorService
	checkService   *service.CheckService
}

func NewScheduler(
	monitorService *service.MonitorService,
	checkService *service.CheckService,
) *Scheduler {
	return &Scheduler{
		monitorService: monitorService,
		checkService:   checkService,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	log.Println("Scheduler started")

	go func() {
		s.runChecks(ctx)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runChecks(ctx)
			case <-ctx.Done():
				log.Println("Scheduler stopped")
				return
			}
		}
	}()
}

func (s *Scheduler) runChecks(ctx context.Context) {
	monitors, err := s.monitorService.GetAll()
	if err != nil {
		log.Println("Failed to load monitors:", err)
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
				log.Println("Failed to check monitor:", err)
			}
		}(monitor)
	}

	wg.Wait()
	log.Println("All checks completed")
}
