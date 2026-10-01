package migrations

import (
	"context"
	"embed"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed sql/*.sql
var embedMigrations embed.FS

func Up(ctx context.Context, pool *pgxpool.Pool) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	db := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, db, "sql"); err != nil {
		return errors.Join(err, db.Close())
	}

	return db.Close()
}
