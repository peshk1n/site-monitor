package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

func (q *queries) OpenIncident(ctx context.Context, id int64) (*models.Incident, error) {
	var i models.Incident
	err := q.tx.GetContext(ctx, &i, "SELECT * FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &i, err
}
func (q *queries) SaveIncident(ctx context.Context, i *models.Incident) error {
	if i.ID == 0 {
		return q.named(ctx, i, `INSERT INTO incidents (project_id,monitor_id,severity,first_observed_at,opened_at,summary)
	VALUES (:project_id,:monitor_id,:severity,:first_observed_at,:opened_at,:summary) RETURNING *`, *i)
	}
	return q.execNamed(ctx, `UPDATE incidents SET severity=:severity,resolved_at=:resolved_at,resolution=:resolution,summary=:summary WHERE id=:id`, *i)
}
func (q *queries) Event(ctx context.Context, e models.Event) error {
	payload, err := json.Marshal(map[string]any{"resolution": e.Incident.Resolution, "summary": e.Incident.Summary})
	if err != nil {
		return err
	}
	var id string
	err = q.tx.GetContext(ctx, &id, `INSERT INTO monitor_events (project_id,monitor_id,incident_id,kind,severity,config_version,deduplication_key,occurred_at,payload)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, e.Incident.ProjectID, e.Incident.MonitorID, e.Incident.ID, e.Kind, e.Incident.Severity, e.ConfigVersion, e.DeduplicationKey, e.OccurredAt, string(payload))
	if err != nil {
		return err
	}
	if e.Incident.Resolution != nil && *e.Incident.Resolution != "recovered" {
		return nil
	}
	_, err = q.tx.ExecContext(ctx, `INSERT INTO notification_deliveries (project_id,event_id,channel_id,channel_kind,destination,secret_ref,status)
	SELECT DISTINCT e.project_id,e.id,c.id,c.kind,c.destination,c.secret_ref,
	CASE WHEN EXISTS (SELECT 1 FROM silences s WHERE s.cancelled_at IS NULL AND s.starts_at<=e.occurred_at AND s.ends_at>e.occurred_at
	AND (s.scope='all' OR EXISTS (SELECT 1 FROM silence_projects sp WHERE sp.silence_id=s.id AND sp.project_id=e.project_id)
	OR EXISTS (SELECT 1 FROM silence_monitors sm WHERE sm.silence_id=s.id AND sm.monitor_id=e.monitor_id))) THEN 'suppressed' ELSE 'pending' END
	FROM monitor_events e JOIN notification_rules nr ON nr.project_id=e.project_id
	JOIN notification_rule_channels rc ON rc.rule_id=nr.id
	JOIN notification_channels c ON c.id=rc.channel_id
	WHERE e.id=$1 AND nr.enabled AND c.enabled AND c.archived_at IS NULL AND e.kind=ANY(nr.event_types) AND e.severity=ANY(nr.severities)
	AND (nr.scope='project' OR EXISTS (SELECT 1 FROM notification_rule_monitors rm WHERE rm.rule_id=nr.id AND rm.monitor_id=e.monitor_id))
	ON CONFLICT (event_id,channel_id) DO NOTHING`, id)
	return err
}
func (q *queries) CancelDeliveries(ctx context.Context, id int64) error {
	_, err := q.tx.ExecContext(ctx, `UPDATE notification_deliveries d SET status='cancelled',lease_token=NULL,lease_owner=NULL,lease_expires_at=NULL
	FROM monitor_events e WHERE d.event_id=e.id AND e.monitor_id=$1 AND d.status IN ('pending','processing')`, id)
	return err
}
func (q *queries) Results(ctx context.Context, id int64, page models.Page) ([]models.Result, error) {
	items := []models.Result{}
	err := q.tx.SelectContext(ctx, &items, `SELECT * FROM check_results WHERE monitor_id=$1 ORDER BY observed_at DESC NULLS LAST,id DESC LIMIT $2 OFFSET $3`, id, page.Limit, page.Offset)
	return items, err
}
func (q *queries) InsertResult(ctx context.Context, c *models.Result) (bool, error) {
	query := `INSERT INTO check_results (monitor_id,run_id,origin,config_version,scheduled_at,started_at,finished_at,observed_at,
	outcome,status_code,response_ms,error_kind,error_message,accepted,discard_reason)
	VALUES (:monitor_id,:run_id,:origin,:config_version,:scheduled_at,:started_at,:finished_at,:observed_at,
	:outcome,:status_code,:response_ms,:error_kind,:error_message,:accepted,:discard_reason)
	ON CONFLICT (run_id) DO NOTHING RETURNING *`
	err := q.named(ctx, c, query, *c)
	if errors.Is(err, service.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
func (q *queries) Incidents(ctx context.Context, project int64, monitor *int64, window *models.Window, open bool, page models.Page) ([]models.Incident, error) {
	var from, to *time.Time
	if window != nil {
		from = &window.From
		to = &window.To
	}
	items := []models.Incident{}
	err := q.tx.SelectContext(ctx, &items, `SELECT * FROM incidents WHERE project_id=$1 AND ($2::bigint IS NULL OR monitor_id=$2)
	AND (NOT $3 OR resolved_at IS NULL) AND ($4::timestamptz IS NULL OR (opened_at<$5 AND (resolved_at IS NULL OR resolved_at>$4)))
	ORDER BY opened_at DESC,id DESC LIMIT $6 OFFSET $7`, project, monitor, open, from, to, page.Limit, page.Offset)
	return items, err
}
func emptySummary() map[string]int64 {
	return map[string]int64{"paused": 0, "awaiting": 0, "stale": 0, "confirming": 0, "up": 0, "degraded": 0, "down": 0}
}
func (q *queries) Summary(ctx context.Context, project int64) (map[string]int64, error) {
	rows := []struct {
		Category string `db:"category"`
		Count    int64  `db:"count"`
	}{}
	err := q.tx.SelectContext(ctx, &rows, `SELECT category,count(*) FROM (`+viewQuery+`) v WHERE project_id=$1 AND archived_at IS NULL GROUP BY category`, project)
	result := emptySummary()
	for _, row := range rows {
		result[row.Category] = row.Count
	}
	return result, err
}
func (q *queries) Chart(ctx context.Context, id int64, window models.Window, bucket time.Duration) ([]models.ChartPoint, error) {
	items := []models.ChartPoint{}
	err := q.tx.SelectContext(ctx, &items, `SELECT to_timestamp(floor(extract(epoch FROM observed_at)/$4)*$4) AS time,
	count(*) AS count,count(*) FILTER (WHERE outcome='failure') AS failures,avg(response_ms)::float8 AS response_ms,config_version
	FROM check_results WHERE monitor_id=$1 AND observed_at >= $2 AND observed_at < $3 AND accepted AND origin<>'legacy'
	AND outcome IN ('success','warning','failure') GROUP BY 1,config_version ORDER BY 1,config_version`, id, window.From, window.To, bucket.Seconds())
	return items, err
}
