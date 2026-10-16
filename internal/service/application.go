package service

import (
	"context"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

type Application struct {
	store Store
	now   func() time.Time
}

func New(store Store) *Application { return &Application{store: store, now: time.Now} }

func access(ctx context.Context, r Repository, actor, project int64, write bool, owner bool) (models.Project, error) {
	if actor <= 0 {
		return models.Project{}, ErrUnauthorized
	}
	lock := ""
	if write {
		lock = "share"
	}
	if owner {
		lock = "update"
	}
	p, err := r.Project(ctx, project, lock)
	if err != nil {
		return p, err
	}
	role, err := r.Role(ctx, actor, project, write)
	if err != nil {
		return p, err
	}
	p.Role = role
	if owner && role != "owner" {
		return p, ErrForbidden
	}
	if write && role != "owner" && role != "editor" {
		return p, ErrForbidden
	}
	return p, nil
}

func (a *Application) Identify(ctx context.Context, id int64, name string) (models.User, error) {
	var user models.User
	if id <= 0 {
		return user, ErrUnauthorized
	}
	name, err := Name(name)
	if err != nil {
		return user, err
	}
	err = a.store.Within(ctx, func(r Repository) error { var err error; user, err = r.User(ctx, id, name); return err })
	return user, err
}
