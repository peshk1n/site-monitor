package repository

import (
	"context"

	"github.com/peshk1n/site-monitor/internal/service"
)

func (q *queries) Creation(ctx context.Context, actor int64, key, operation, hash string) ([]byte, error) {
	_, err := q.tx.ExecContext(ctx, `INSERT INTO creation_requests (user_id,request_key,operation,request_hash)
	VALUES ($1,$2,$3,$4) ON CONFLICT (user_id,request_key) DO NOTHING`, actor, key, operation, hash)
	if err != nil {
		return nil, err
	}
	var row struct {
		Operation string `db:"operation"`
		Hash      string `db:"request_hash"`
		Response  []byte `db:"response"`
	}
	err = q.tx.GetContext(ctx, &row, `SELECT operation,request_hash,response FROM creation_requests WHERE user_id=$1 AND request_key=$2 FOR UPDATE`, actor, key)
	if err != nil {
		return nil, err
	}
	if row.Operation != operation || row.Hash != hash {
		return nil, service.ErrConflict
	}
	return row.Response, nil
}
func (q *queries) SaveCreation(ctx context.Context, actor int64, key string, response []byte) error {
	_, err := q.tx.ExecContext(ctx, `UPDATE creation_requests SET response=$3 WHERE user_id=$1 AND request_key=$2`, actor, key, string(response))
	return err
}
