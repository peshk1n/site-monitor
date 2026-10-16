package repository

import (
	"context"

	"github.com/lib/pq"
	"github.com/peshk1n/site-monitor/internal/models"
)

type monitorRow struct {
	models.Monitor
	Codes pq.Int64Array `db:"expected_status_codes"`
}
type viewRow struct {
	models.MonitorView
	Codes pq.Int64Array `db:"expected_status_codes"`
}

func (row monitorRow) value() models.Monitor {
	row.Monitor.ExpectedStatusCodes = []int64(row.Codes)
	return row.Monitor
}
func (row viewRow) value() models.MonitorView {
	row.MonitorView.ExpectedStatusCodes = []int64(row.Codes)
	return row.MonitorView
}

const viewQuery = `SELECT m.*, r.health_status,r.last_observed_at,r.fresh_until,r.candidate_status,r.candidate_count,
	i.id AS open_incident_id,c.outcome AS last_outcome,c.response_ms AS last_response_ms,
	CASE WHEN m.archived_at IS NOT NULL OR p.archived_at IS NOT NULL THEN 'archived'
	WHEN NOT m.is_active THEN 'paused'
	WHEN r.last_observed_at IS NULL THEN CASE WHEN CURRENT_TIMESTAMP > r.evaluation_started_at + (2*m.interval_seconds+m.timeout_seconds)*INTERVAL '1 second' THEN 'stale' ELSE 'awaiting' END
	WHEN CURRENT_TIMESTAMP > r.fresh_until THEN 'stale'
	WHEN r.health_status='unknown' THEN 'confirming' ELSE r.health_status END AS category
	FROM monitors m JOIN projects p ON p.id=m.project_id JOIN monitor_runtime r ON r.monitor_id=m.id
	LEFT JOIN incidents i ON i.monitor_id=m.id AND i.resolved_at IS NULL
	LEFT JOIN LATERAL (SELECT outcome,response_ms FROM check_results WHERE monitor_id=m.id AND accepted ORDER BY recorded_at DESC,id DESC LIMIT 1) c ON true`

func (q *queries) Monitors(ctx context.Context, project int64, filter models.MonitorFilter) ([]models.MonitorView, error) {
	rows := []viewRow{}
	err := q.tx.SelectContext(ctx, &rows, `SELECT * FROM (`+viewQuery+`) v WHERE project_id=$1 AND (archived_at IS NOT NULL)=$2
	AND ($3='' OR strpos(lower(name),lower($3))>0 OR strpos(lower(url),lower($3))>0)
	AND ($4='' OR category=$4) ORDER BY id LIMIT $5 OFFSET $6`, project, filter.Archived, filter.Search, filter.Category, filter.Page.Limit, filter.Page.Offset)
	items := make([]models.MonitorView, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.value())
	}
	return items, err
}
func (q *queries) Monitor(ctx context.Context, project, id int64, lock bool) (models.Monitor, error) {
	query := "SELECT * FROM monitors WHERE id=$1 AND project_id=$2"
	if lock {
		query += " FOR UPDATE"
	}
	var row monitorRow
	err := q.tx.GetContext(ctx, &row, query, id, project)
	return row.value(), translate(err)
}
func (q *queries) MonitorView(ctx context.Context, project, id int64) (models.MonitorView, error) {
	var row viewRow
	err := q.tx.GetContext(ctx, &row, viewQuery+" WHERE m.project_id=$1 AND m.id=$2", project, id)
	return row.value(), translate(err)
}

func monitorParams(m models.Monitor) map[string]any {
	return map[string]any{
		"id": m.ID, "project_id": m.ProjectID, "name": m.Name, "url": m.URL, "interval": m.IntervalSeconds, "timeout": m.TimeoutSeconds,
		"codes": pq.Array(m.ExpectedStatusCodes), "match": m.BodyMatchType, "pattern": m.BodyPattern, "warning": m.WarningResponseMS,
		"failure": m.FailureThreshold, "recovery": m.RecoveryThreshold, "body": m.MaxBodyBytes, "redirects": m.MaxRedirects,
		"active": m.IsActive, "archived": m.ArchivedAt,
	}
}
func (q *queries) CreateMonitor(ctx context.Context, m *models.Monitor) error {
	var row monitorRow
	err := q.named(ctx, &row, `INSERT INTO monitors (project_id,name,url,interval_seconds,timeout_seconds,expected_status_codes,
	body_match_type,body_pattern,warning_response_ms,failure_threshold,recovery_threshold,max_body_bytes,max_redirects,is_active)
	VALUES (:project_id,:name,:url,:interval,:timeout,:codes,:match,:pattern,:warning,:failure,:recovery,:body,:redirects,:active) RETURNING *`, monitorParams(*m))
	if err != nil {
		return err
	}
	*m = row.value()
	_, err = q.tx.ExecContext(ctx, `INSERT INTO monitor_runtime (monitor_id,state_config_version) VALUES ($1,$2)`, m.ID, m.ConfigVersion)
	return err
}
func (q *queries) SaveMonitor(ctx context.Context, m *models.Monitor) error {
	var row monitorRow
	err := q.named(ctx, &row, `UPDATE monitors SET name=:name,url=:url,interval_seconds=:interval,timeout_seconds=:timeout,
	expected_status_codes=:codes,body_match_type=:match,body_pattern=:pattern,warning_response_ms=:warning,failure_threshold=:failure,
	recovery_threshold=:recovery,max_body_bytes=:body,max_redirects=:redirects,is_active=:active,archived_at=:archived WHERE id=:id RETURNING *`, monitorParams(*m))
	if err == nil {
		*m = row.value()
	}
	return err
}
func (q *queries) Runtime(ctx context.Context, id int64, lock bool) (models.Runtime, error) {
	query := "SELECT * FROM monitor_runtime WHERE monitor_id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	var r models.Runtime
	err := q.tx.GetContext(ctx, &r, query, id)
	return r, translate(err)
}
func (q *queries) SaveRuntime(ctx context.Context, r models.Runtime) error {
	return q.execNamed(ctx, `UPDATE monitor_runtime SET next_run_at=:next_run_at,active_run_id=:active_run_id,
	lease_owner=:lease_owner,lease_expires_at=:lease_expires_at,running_config_version=:running_config_version,
	health_status=:health_status,state_config_version=:state_config_version,last_check_id=:last_check_id,
	last_observed_at=:last_observed_at,fresh_until=:fresh_until,state_changed_at=:state_changed_at,updated_at=:updated_at,
	evaluation_started_at=:evaluation_started_at,candidate_status=:candidate_status,candidate_count=:candidate_count,
	candidate_since=:candidate_since WHERE monitor_id=:monitor_id`, r)
}
