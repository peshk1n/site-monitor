package service

import (
	"context"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

func (a *Application) Monitors(ctx context.Context, actor, project int64, filter models.MonitorFilter) ([]models.MonitorView, error) {
	var items []models.MonitorView
	if !ValidPage(filter.Page) || len(filter.Search) > 200 {
		return nil, Invalid("invalid filter")
	}
	switch filter.Category {
	case "", "paused", "awaiting", "stale", "confirming", "up", "degraded", "down":
	default:
		return nil, Invalid("invalid category")
	}
	err := a.store.View(ctx, func(r Repository) error {
		if _, err := access(ctx, r, actor, project, false, false); err != nil {
			return err
		}
		var err error
		items, err = r.Monitors(ctx, project, filter)
		return err
	})
	return items, err
}

func (a *Application) Monitor(ctx context.Context, actor, project, id int64) (models.MonitorView, error) {
	var m models.MonitorView
	err := a.store.View(ctx, func(r Repository) error {
		if _, err := access(ctx, r, actor, project, false, false); err != nil {
			return err
		}
		var err error
		m, err = r.MonitorView(ctx, project, id)
		return err
	})
	return m, err
}

func (a *Application) CreateMonitor(ctx context.Context, actor, project int64, c models.MonitorConfig, requestKeys ...string) (models.Monitor, error) {
	m := models.Monitor{MonitorConfig: c, ProjectID: project, IsActive: true}
	if err := ValidateConfig(&m.MonitorConfig); err != nil {
		return m, err
	}
	key, err := creationKey(requestKeys)
	if err != nil {
		return m, err
	}
	err = a.store.Within(ctx, func(r Repository) error {
		p, err := access(ctx, r, actor, project, true, false)
		if err != nil {
			return err
		}
		if p.ArchivedAt != nil {
			return ErrConflict
		}
		input := struct {
			Project int64
			Config  models.MonitorConfig
		}{project, m.MonitorConfig}
		if cached, err := cachedCreation(ctx, r, actor, key, "create_monitor", input, &m); cached || err != nil {
			return err
		}
		if err = r.CreateMonitor(ctx, &m); err != nil {
			return err
		}
		if err = r.Audit(ctx, actor, project, "monitor_created", "monitor", m.ID); err != nil {
			return err
		}
		return saveCreation(ctx, r, actor, key, m)
	})
	return m, err
}

func (a *Application) UpdateMonitor(ctx context.Context, actor, project, id, version int64, c models.MonitorConfig) (models.Monitor, error) {
	if version <= 0 {
		return models.Monitor{}, Invalid("config_version is required")
	}
	if err := ValidateConfig(&c); err != nil {
		return models.Monitor{}, err
	}
	var m models.Monitor
	err := a.store.Within(ctx, func(r Repository) error {
		p, err := access(ctx, r, actor, project, true, false)
		if err != nil {
			return err
		}
		if p.ArchivedAt != nil {
			return ErrConflict
		}
		m, err = r.Monitor(ctx, project, id, true)
		if err != nil {
			return err
		}
		if m.ArchivedAt != nil || m.ConfigVersion != version {
			return ErrConflict
		}
		changed := !sameConditions(m.MonitorConfig, c)
		m.MonitorConfig = c
		if err = r.SaveMonitor(ctx, &m); err != nil {
			return err
		}
		if changed {
			now := a.now().UTC()
			state, err := r.Runtime(ctx, id, true)
			if err != nil {
				return err
			}
			reset(&state, m, now, true)
			if err = a.closeIncident(ctx, r, m, "reconfigured", now); err != nil {
				return err
			}
			if err = r.SaveRuntime(ctx, state); err != nil {
				return err
			}
			if err = r.CancelDeliveries(ctx, id); err != nil {
				return err
			}
		}
		return r.Audit(ctx, actor, project, "monitor_updated", "monitor", id)
	})
	return m, err
}

func reset(state *models.Runtime, m models.Monitor, now time.Time, unknown bool) {
	state.Release()
	state.ClearCandidate()
	state.LastCheckID = nil
	state.LastObservedAt = nil
	state.FreshUntil = nil
	state.EvaluationStartedAt = now
	state.NextRunAt = now
	state.StateConfigVersion = m.ConfigVersion
	state.UpdatedAt = now
	if unknown && state.HealthStatus != "unknown" {
		state.HealthStatus = "unknown"
		state.StateChangedAt = now
	}
}

func (a *Application) ChangeMonitor(ctx context.Context, actor, project, id, version int64, action string) (models.Monitor, error) {
	var m models.Monitor
	if version <= 0 {
		return m, Invalid("config_version is required")
	}
	err := a.store.Within(ctx, func(r Repository) error {
		p, err := access(ctx, r, actor, project, true, false)
		if err != nil {
			return err
		}
		if p.ArchivedAt != nil {
			return ErrConflict
		}
		m, err = r.Monitor(ctx, project, id, true)
		if err != nil {
			return err
		}
		if m.ConfigVersion != version {
			return ErrConflict
		}
		state, err := r.Runtime(ctx, id, true)
		if err != nil {
			return err
		}
		now := a.now().UTC()
		switch action {
		case "pause":
			if m.ArchivedAt != nil {
				return ErrConflict
			}
			if !m.IsActive {
				return nil
			}
			m.IsActive = false
		case "resume":
			if m.ArchivedAt != nil {
				return ErrConflict
			}
			if m.IsActive {
				return nil
			}
			m.IsActive = true
		case "archive":
			if m.ArchivedAt != nil {
				return nil
			}
			m.ArchivedAt = &now
			m.IsActive = false
		case "restore":
			if m.ArchivedAt == nil {
				return nil
			}
			m.ArchivedAt = nil
			m.IsActive = false
		default:
			return Invalid("invalid monitor action")
		}
		if err = r.SaveMonitor(ctx, &m); err != nil {
			return err
		}
		if action == "pause" {
			state.Release()
			state.ClearCandidate()
			state.StateConfigVersion = m.ConfigVersion
			state.UpdatedAt = now
		} else {
			reset(&state, m, now, action != "resume")
		}
		if action == "archive" {
			if err = a.closeIncident(ctx, r, m, "archived", now); err != nil {
				return err
			}
		}
		if err = r.SaveRuntime(ctx, state); err != nil {
			return err
		}
		if err = r.CancelDeliveries(ctx, id); err != nil {
			return err
		}
		return r.Audit(ctx, actor, project, "monitor_"+action, "monitor", id)
	})
	return m, err
}
