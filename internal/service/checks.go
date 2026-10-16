package service

import (
	"context"
	"fmt"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

func (a *Application) Claim(ctx context.Context, owner string) (*models.Job, error) {
	return a.store.Claim(ctx, owner)
}

func (a *Application) Complete(ctx context.Context, job models.Job, result models.Result) error {
	if result.ObservedAt == nil || result.StartedAt == nil || result.FinishedAt == nil || result.ResponseMS == nil {
		return Invalid("incomplete observation")
	}
	return a.store.Within(ctx, func(r Repository) error {
		p, err := r.Project(ctx, job.Monitor.ProjectID, "share")
		if err != nil {
			return err
		}
		m, err := r.Monitor(ctx, p.ID, job.Monitor.ID, true)
		if err != nil {
			return err
		}
		state, err := r.Runtime(ctx, m.ID, true)
		if err != nil {
			return err
		}
		now := a.now().UTC()
		result.MonitorID = m.ID
		result.RunID = job.RunID
		result.Origin = "scheduled"
		result.ConfigVersion = &job.Monitor.ConfigVersion
		result.ScheduledAt = &job.ScheduledAt
		reason := ""
		switch {
		case m.ConfigVersion != job.Monitor.ConfigVersion:
			reason = "configuration_changed"
		case p.ArchivedAt != nil || m.ArchivedAt != nil || !m.IsActive:
			reason = "monitor_inactive"
		case state.ActiveRunID == nil || *state.ActiveRunID != job.RunID || state.LeaseExpiresAt == nil || !now.Before(*state.LeaseExpiresAt):
			reason = "lease_lost"
		case state.LastObservedAt != nil && result.ObservedAt.Before(*state.LastObservedAt):
			reason = "older_observation"
		}
		result.Accepted = reason == ""
		if reason != "" {
			result.DiscardReason = &reason
		}
		inserted, err := r.InsertResult(ctx, &result)
		if err != nil || !inserted {
			return err
		}
		if !result.Accepted {
			return nil
		}
		first := Advance(&state, m, result, now)
		state.Release()
		state.StateConfigVersion = m.ConfigVersion
		state.UpdatedAt = now
		state.NextRunAt = result.StartedAt.Add(time.Duration(m.IntervalSeconds) * time.Second)
		if state.NextRunAt.Before(now) {
			state.NextRunAt = now
		}
		if err = r.SaveRuntime(ctx, state); err != nil {
			return err
		}
		if first == nil {
			return nil
		}
		if state.HealthStatus == "up" {
			return a.closeIncident(ctx, r, m, "recovered", now)
		}
		severity := "warning"
		if state.HealthStatus == "down" {
			severity = "critical"
		}
		i, err := r.OpenIncident(ctx, m.ID)
		if err != nil {
			return err
		}
		kind := "severity_changed"
		if i == nil {
			kind = "incident_opened"
			i = &models.Incident{ProjectID: p.ID, MonitorID: m.ID, FirstObservedAt: *first, OpenedAt: now}
		}
		i.Severity = severity
		i.Summary = "Service is " + state.HealthStatus
		if err = r.SaveIncident(ctx, i); err != nil {
			return err
		}
		return r.Event(ctx, models.Event{Incident: *i, Kind: kind, ConfigVersion: m.ConfigVersion, OccurredAt: now, DeduplicationKey: fmt.Sprintf("%s:%s", job.RunID, kind)})
	})
}
