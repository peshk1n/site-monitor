package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type Scheduler struct {
	monitorService *service.MonitorService
	checkService   *service.CheckService
	stopChan       chan struct{}
}

func NewScheduler(
	monitorService *service.MonitorService,
	checkService *service.CheckService,
) *Scheduler {
	return &Scheduler{
		monitorService: monitorService,
		checkService:   checkService,
		stopChan:       make(chan struct{}),
	}
}
func (s *Scheduler) Start() {
	log.Println("Scheduler started")

	go func() {
		s.runChecks()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runChecks()
			case <-s.stopChan:
				log.Println("Scheduler stopped")
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
}

func (s *Scheduler) runChecks() {
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
			// Теперь вся логика внутри сервиса
			if err := s.checkService.RunCheck(m); err != nil {
				log.Println("Failed to check monitor:", err)
			}
		}(monitor)
	}

	wg.Wait()
	log.Println("All checks completed")
}
