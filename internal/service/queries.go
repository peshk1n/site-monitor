package service

import (
	"context"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

func (a *Application) Results(ctx context.Context, actor, project, id int64, page models.Page) ([]models.Result, error) {
	if !ValidPage(page) {
		return nil, Invalid("invalid pagination")
	}
	var items []models.Result
	err := a.store.View(ctx, func(r Repository) error {
		if _, err := access(ctx, r, actor, project, false, false); err != nil {
			return err
		}
		if _, err := r.Monitor(ctx, project, id, false); err != nil {
			return err
		}
		var err error
		items, err = r.Results(ctx, id, page)
		return err
	})
	return items, err
}

func (a *Application) Incidents(ctx context.Context, actor, project int64, monitor *int64, page models.Page) ([]models.Incident, error) {
	if !ValidPage(page) {
		return nil, Invalid("invalid pagination")
	}
	var items []models.Incident
	err := a.store.View(ctx, func(r Repository) error {
		if _, err := access(ctx, r, actor, project, false, false); err != nil {
			return err
		}
		if monitor != nil {
			if _, err := r.Monitor(ctx, project, *monitor, false); err != nil {
				return err
			}
		}
		var err error
		items, err = r.Incidents(ctx, project, monitor, nil, false, page)
		return err
	})
	return items, err
}

func (a *Application) Dashboard(ctx context.Context, actor, project int64, monitor *int64, window models.Window) (models.Dashboard, error) {
	now := a.now().UTC()
	d := models.Dashboard{AsOf: now, From: window.From, To: window.To, Chart: []models.ChartPoint{}, ChartMonitorID: monitor}
	span := window.To.Sub(window.From)
	if span <= 0 || span > 30*24*time.Hour || window.To.After(now.Add(time.Minute)) {
		return d, Invalid("dashboard period must be between 0 and 30 days and not in the future")
	}
	err := a.store.View(ctx, func(r Repository) error {
		var err error
		d.Project, err = access(ctx, r, actor, project, false, false)
		if err != nil {
			return err
		}
		d.Summary, err = r.Summary(ctx, project)
		if err != nil {
			return err
		}
		d.OpenIncidents, err = r.Incidents(ctx, project, nil, nil, true, models.Page{Limit: 101})
		if err != nil {
			return err
		}
		if len(d.OpenIncidents) > 100 {
			d.OpenIncidentsHasMore = true
			d.OpenIncidents = d.OpenIncidents[:100]
		}
		d.Incidents, err = r.Incidents(ctx, project, nil, &window, false, models.Page{Limit: 101})
		if err != nil {
			return err
		}
		if len(d.Incidents) > 100 {
			d.IncidentsHasMore = true
			d.Incidents = d.Incidents[:100]
		}
		if monitor != nil {
			if _, err = r.Monitor(ctx, project, *monitor, false); err != nil {
				return err
			}
			bucket := time.Duration(int64(span/time.Second)/300+1) * time.Second
			d.Chart, err = r.Chart(ctx, *monitor, window, bucket)
		}
		return err
	})
	return d, err
}
