package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

// withTransaction keeps the repository usable with focused tests that provide
// a transaction or a lightweight DBTX, while using a real SQL transaction for
// the production *sql.DB passed by app.NewAPI.
func (r *Repository) withTransaction(ctx context.Context, fn func(*sqlc.Queries) error) (err error) {
	beginner, ok := r.exec.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return fn(r.queries)
	}

	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	if err = fn(r.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
