package service

import (
	"context"
	"fmt"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

func (a *Application) closeIncident(ctx context.Context, r Repository, m models.Monitor, reason string, now time.Time) error {
	i, err := r.OpenIncident(ctx, m.ID)
	if err != nil || i == nil {
		return err
	}
	i.ResolvedAt = &now
	i.Resolution = &reason
	if err = r.SaveIncident(ctx, i); err != nil {
		return err
	}
	return r.Event(ctx, models.Event{Incident: *i, Kind: "incident_resolved", ConfigVersion: m.ConfigVersion, OccurredAt: now, DeduplicationKey: fmt.Sprintf("close:%d:%s", i.ID, reason)})
}
