package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

func (s *Store) Claim(ctx context.Context, owner string) (*models.Job, error) {
	candidates := []struct {
		MonitorID int64 `db:"monitor_id"`
		ProjectID int64 `db:"project_id"`
	}{}
	err := s.db.SelectContext(ctx, &candidates, `SELECT r.monitor_id,m.project_id FROM monitor_runtime r
	JOIN monitors m ON m.id=r.monitor_id JOIN projects p ON p.id=m.project_id
	WHERE m.is_active AND m.archived_at IS NULL AND p.archived_at IS NULL AND r.next_run_at<=CURRENT_TIMESTAMP
	AND (r.active_run_id IS NULL OR r.lease_expires_at<=CURRENT_TIMESTAMP) ORDER BY r.next_run_at,r.monitor_id LIMIT 32`)
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		var job *models.Job
		err = s.Within(ctx, func(repo service.Repository) error {
			q := repo.(*queries)
			p, err := q.Project(ctx, candidate.ProjectID, "share")
			if err != nil {
				return err
			}
			if p.ArchivedAt != nil {
				return nil
			}
			var row monitorRow
			err = q.tx.GetContext(ctx, &row, `SELECT * FROM monitors WHERE id=$1 AND is_active AND archived_at IS NULL FOR UPDATE SKIP LOCKED`, candidate.MonitorID)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			var state models.Runtime
			err = q.tx.GetContext(ctx, &state, `SELECT * FROM monitor_runtime WHERE monitor_id=$1 AND next_run_at<=CURRENT_TIMESTAMP
			AND (active_run_id IS NULL OR lease_expires_at<=CURRENT_TIMESTAMP) FOR UPDATE SKIP LOCKED`, candidate.MonitorID)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			var runID string
			err = q.tx.GetContext(ctx, &runID, `UPDATE monitor_runtime SET active_run_id=gen_random_uuid(),lease_owner=$2,
			lease_expires_at=CURRENT_TIMESTAMP + $3*INTERVAL '1 second',running_config_version=$4,updated_at=CURRENT_TIMESTAMP
			WHERE monitor_id=$1 RETURNING active_run_id`, candidate.MonitorID, owner, row.TimeoutSeconds+30, row.ConfigVersion)
			if err != nil {
				return err
			}
			job = &models.Job{Monitor: row.value(), RunID: runID, ScheduledAt: state.NextRunAt}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if job != nil {
			return job, nil
		}
	}
	return nil, nil
}
