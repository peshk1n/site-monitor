package scheduler

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

type jobs struct {
	mu        sync.Mutex
	claimed   int
	completed int
}

func (j *jobs) Claim(ctx context.Context, owner string) (*models.Job, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.claimed++
	return &models.Job{}, nil
}
func (j *jobs) Complete(ctx context.Context, job models.Job, result models.Result) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	j.completed++
	return nil
}

type blockingProbe struct{ started chan struct{} }

func (p blockingProbe) Run(ctx context.Context, job models.Job) models.Result {
	p.started <- struct{}{}
	<-ctx.Done()
	return models.Result{Outcome: "cancelled"}
}
func TestShutdownWaitsForWorkersAndPersistsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	j := &jobs{}
	p := blockingProbe{started: make(chan struct{}, 2)}
	s := New(j, p, slog.New(slog.NewTextHandler(io.Discard, nil)), 2, "test")
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	for i := 0; i < 2; i++ {
		select {
		case <-p.started:
		case <-time.After(time.Second):
			t.Fatal("worker did not start")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("workers did not stop")
	}
	if j.completed != 2 || j.claimed != 2 {
		t.Fatalf("unexpected work on shutdown: %+v", j)
	}
}
