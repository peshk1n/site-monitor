package scheduler

import (
	"log"
	"net/http"
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
			s.checkMonitor(m)
		}(monitor)
	}

	wg.Wait()

	log.Println("All checks completed")
}

func (s *Scheduler) checkMonitor(monitor models.Monitor) {

	client := &http.Client{
		Timeout: time.Duration(monitor.Timeout) * time.Second,
	}

	start := time.Now()

	resp, err := client.Get(monitor.URL)

	responseMs := int(time.Since(start).Milliseconds())

	check := &models.Check{
		MonitorID:  monitor.ID,
		ResponseMs: responseMs,
	}

	if err != nil {

		check.IsUp = false
		check.Error = err.Error()

		log.Printf("%s is DOWN: %s\n", monitor.URL, err.Error())

	} else {

		defer resp.Body.Close()

		check.StatusCode = resp.StatusCode
		check.IsUp = resp.StatusCode < 400

		if !check.IsUp {

			check.Error = "HTTP " + http.StatusText(resp.StatusCode)

			log.Printf("%s returned %d\n",
				monitor.URL,
				resp.StatusCode,
			)

		} else {

			log.Printf("%s is UP (%d ms)\n",
				monitor.URL,
				responseMs,
			)

		}
	}

	if err := s.checkService.Create(check); err != nil {
		log.Println("Failed to save check result:", err)
	}
}
