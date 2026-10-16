package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

type Jobs interface {
	Claim(context.Context, string) (*models.Job, error)
	Complete(context.Context, models.Job, models.Result) error
}
type Probe interface {
	Run(context.Context, models.Job) models.Result
}
type Scheduler struct {
	jobs    Jobs
	probe   Probe
	log     *slog.Logger
	workers int
	owner   string
}

func New(jobs Jobs, probe Probe, log *slog.Logger, workers int, owner string) *Scheduler {
	return &Scheduler{jobs: jobs, probe: probe, log: log, workers: workers, owner: owner}
}
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func(index int) { defer wg.Done(); s.worker(ctx, fmt.Sprintf("%s/%d", s.owner, index)) }(i)
	}
	wg.Wait()
}
func (s *Scheduler) worker(ctx context.Context, owner string) {
	for ctx.Err() == nil {
		claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		job, err := s.jobs.Claim(claimCtx, owner)
		cancel()
		if err != nil {
			s.log.Error("claim failed", "error", err)
		}
		if err != nil || job == nil {
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		result := s.probe.Run(ctx, *job)
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err = s.jobs.Complete(finishCtx, *job, result)
		cancel()
		if err != nil {
			s.log.Error("result persistence failed", "monitor_id", job.Monitor.ID, "error", err)
		}
	}
}
