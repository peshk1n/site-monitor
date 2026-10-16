package service

import (
	"context"

	"github.com/peshk1n/site-monitor/internal/models"
)

func (a *Application) Projects(ctx context.Context, actor int64, archived bool, page models.Page) ([]models.Project, error) {
	if actor <= 0 {
		return nil, ErrUnauthorized
	}
	if !ValidPage(page) {
		return nil, Invalid("invalid pagination")
	}
	var projects []models.Project
	err := a.store.View(ctx, func(r Repository) error {
		var err error
		projects, err = r.Projects(ctx, actor, archived, page)
		return err
	})
	return projects, err
}

func (a *Application) Project(ctx context.Context, actor, id int64) (models.Project, error) {
	var p models.Project
	err := a.store.View(ctx, func(r Repository) error { var err error; p, err = access(ctx, r, actor, id, false, false); return err })
	return p, err
}

func (a *Application) CreateProject(ctx context.Context, actor int64, name string, requestKeys ...string) (models.Project, error) {
	var p models.Project
	if actor <= 0 {
		return p, ErrUnauthorized
	}
	name, err := Name(name)
	if err != nil {
		return p, err
	}
	key, err := creationKey(requestKeys)
	if err != nil {
		return p, err
	}
	err = a.store.Within(ctx, func(r Repository) error {
		if cached, err := cachedCreation(ctx, r, actor, key, "create_project", name, &p); cached || err != nil {
			if err != nil {
				return err
			}
			_, err = access(ctx, r, actor, p.ID, false, false)
			return err
		}
		var err error
		p, err = r.CreateProject(ctx, actor, name)
		if err != nil {
			return err
		}
		if err = r.Audit(ctx, actor, p.ID, "project_created", "project", p.ID); err != nil {
			return err
		}
		return saveCreation(ctx, r, actor, key, p)
	})
	return p, err
}

func (a *Application) ChangeProject(ctx context.Context, actor, id int64, action, name string) (models.Project, error) {
	var p models.Project
	if action == "rename" {
		var err error
		name, err = Name(name)
		if err != nil {
			return p, err
		}
	}
	err := a.store.Within(ctx, func(r Repository) error {
		var err error
		p, err = access(ctx, r, actor, id, true, true)
		if err != nil {
			return err
		}
		now := a.now().UTC()
		switch action {
		case "rename":
			if p.ArchivedAt != nil {
				return ErrConflict
			}
			p.Name = name
		case "archive", "restore":
			if (action == "archive") == (p.ArchivedAt != nil) {
				return nil
			}
			for offset := 0; ; offset += 100 {
				items, err := r.Monitors(ctx, id, models.MonitorFilter{Page: models.Page{Limit: 100, Offset: offset}})
				if err != nil {
					return err
				}
				for _, item := range items {
					m, err := r.Monitor(ctx, id, item.ID, true)
					if err != nil {
						return err
					}
					state, err := r.Runtime(ctx, m.ID, true)
					if err != nil {
						return err
					}
					m.IsActive = false
					if err = r.SaveMonitor(ctx, &m); err != nil {
						return err
					}
					if err = a.closeIncident(ctx, r, m, "archived", now); err != nil {
						return err
					}
					reset(&state, m, now, true)
					if err = r.SaveRuntime(ctx, state); err != nil {
						return err
					}
					if err = r.CancelDeliveries(ctx, m.ID); err != nil {
						return err
					}
				}
				if len(items) < 100 {
					break
				}
			}
			if action == "archive" {
				p.ArchivedAt = &now
			} else {
				p.ArchivedAt = nil
			}
		default:
			return Invalid("invalid project action")
		}
		p.UpdatedAt = now
		if err = r.SaveProject(ctx, p); err != nil {
			return err
		}
		return r.Audit(ctx, actor, id, "project_"+action, "project", id)
	})
	return p, err
}
