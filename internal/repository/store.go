package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/service"
)

type Store struct{ db *sqlx.DB }
type queries struct{ tx *sqlx.Tx }

func New(db *sqlx.DB) *Store { return &Store{db: db} }
func translate(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrNotFound
	}
	var pg *pq.Error
	if errors.As(err, &pg) && (pg.Code == "40001" || pg.Code == "40P01" || pg.Code == "23505") {
		return service.ErrConflict
	}
	return err
}
func (s *Store) transaction(ctx context.Context, read bool, fn func(service.Repository) error) error {
	options := &sql.TxOptions{}
	if read {
		options.ReadOnly = true
		options.Isolation = sql.LevelRepeatableRead
	}
	tx, err := s.db.BeginTxx(ctx, options)
	if err != nil {
		return translate(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SET LOCAL lock_timeout = '5s'"); err != nil {
		return err
	}
	if err = fn(&queries{tx: tx}); err != nil {
		return translate(err)
	}
	return translate(tx.Commit())
}
func (s *Store) Within(ctx context.Context, fn func(service.Repository) error) error {
	return s.transaction(ctx, false, fn)
}
func (s *Store) View(ctx context.Context, fn func(service.Repository) error) error {
	return s.transaction(ctx, true, fn)
}

func (q *queries) named(ctx context.Context, dest any, statement string, arg any) error {
	query, args, err := sqlx.Named(statement, arg)
	if err != nil {
		return err
	}
	return translate(q.tx.GetContext(ctx, dest, q.tx.Rebind(query), args...))
}
func (q *queries) execNamed(ctx context.Context, statement string, arg any) error {
	_, err := q.tx.NamedExecContext(ctx, statement, arg)
	return translate(err)
}

func (q *queries) User(ctx context.Context, id int64, name string) (models.User, error) {
	var u models.User
	err := q.tx.GetContext(ctx, &u, `INSERT INTO users (login,display_name,telegram_user_id)
	VALUES ('tg-' || gen_random_uuid()::text,$2,$1)
	ON CONFLICT (telegram_user_id) DO UPDATE SET display_name=EXCLUDED.display_name,updated_at=CURRENT_TIMESTAMP
	WHERE users.archived_at IS NULL RETURNING id,telegram_user_id,display_name`, id, name)
	if errors.Is(err, sql.ErrNoRows) {
		return u, service.ErrUnauthorized
	}
	return u, err
}

func (q *queries) Projects(ctx context.Context, actor int64, archived bool, page models.Page) ([]models.Project, error) {
	items := []models.Project{}
	err := q.tx.SelectContext(ctx, &items, `SELECT p.*,pm.role FROM projects p
	JOIN project_members pm ON pm.project_id=p.id JOIN users u ON u.id=pm.user_id
	WHERE pm.user_id=$1 AND u.archived_at IS NULL AND (p.archived_at IS NOT NULL)=$2
	ORDER BY p.name,p.id LIMIT $3 OFFSET $4`, actor, archived, page.Limit, page.Offset)
	if err != nil || len(items) == 0 {
		return items, err
	}
	ids := make([]int64, len(items))
	byID := map[int64]int{}
	for index := range items {
		ids[index] = items[index].ID
		byID[items[index].ID] = index
		items[index].Summary = emptySummary()
	}
	var counts []struct {
		ProjectID int64  `db:"project_id"`
		Category  string `db:"category"`
		Count     int64  `db:"count"`
	}
	err = q.tx.SelectContext(ctx, &counts, `SELECT project_id,category,count(*) FROM (`+viewQuery+`) v WHERE project_id=ANY($1) AND archived_at IS NULL GROUP BY project_id,category`, pq.Array(ids))
	for _, row := range counts {
		items[byID[row.ProjectID]].Summary[row.Category] = row.Count
	}
	return items, err
}

func (q *queries) Project(ctx context.Context, id int64, lock string) (models.Project, error) {
	query := "SELECT * FROM projects WHERE id=$1"
	if lock == "share" {
		query += " FOR SHARE"
	}
	if lock == "update" {
		query += " FOR UPDATE"
	}
	var p models.Project
	err := q.tx.GetContext(ctx, &p, query, id)
	return p, translate(err)
}
func (q *queries) Role(ctx context.Context, actor, project int64, lock bool) (string, error) {
	query := `SELECT pm.role FROM project_members pm JOIN users u ON u.id=pm.user_id
	WHERE pm.user_id=$1 AND pm.project_id=$2 AND u.archived_at IS NULL`
	if lock {
		query += " FOR SHARE OF pm,u"
	}
	var role string
	err := q.tx.GetContext(ctx, &role, query, actor, project)
	return role, translate(err)
}
func (q *queries) CreateProject(ctx context.Context, actor int64, name string) (models.Project, error) {
	var p models.Project
	err := q.tx.GetContext(ctx, &p, `INSERT INTO projects (slug,name) SELECT 'p-' || gen_random_uuid()::text,$2
	FROM users WHERE id=$1 AND archived_at IS NULL RETURNING *`, actor, name)
	if err != nil {
		return p, translate(err)
	}
	_, err = q.tx.ExecContext(ctx, `INSERT INTO project_members (project_id,user_id,role,granted_by) VALUES ($1,$2,'owner',$2)`, p.ID, actor)
	p.Role = "owner"
	return p, err
}
func (q *queries) SaveProject(ctx context.Context, p models.Project) error {
	return q.execNamed(ctx, `UPDATE projects SET name=:name,archived_at=:archived_at,updated_at=:updated_at WHERE id=:id`, p)
}
func (q *queries) Audit(ctx context.Context, actor, project int64, action, entity string, id int64) error {
	_, err := q.tx.ExecContext(ctx, `INSERT INTO audit_log (project_id,actor_id,action,entity_type,entity_id) VALUES ($1,$2,$3,$4,$5)`, project, actor, action, entity, id)
	return err
}
