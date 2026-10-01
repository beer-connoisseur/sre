package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"urlshort/internal/entity"
)

const (
	pgUniqueViolation      = "23505"
	pgInvalidTextRepresent = "22P02"
	codeUniqueConstraint   = "links_code_key"
)

type LinkRepository struct {
	db *pgxpool.Pool
}

func NewLinkRepository(db *pgxpool.Pool) *LinkRepository {
	return &LinkRepository{db: db}
}

func (r *LinkRepository) Create(ctx context.Context, link entity.Link) (entity.Link, error) {
	const query = `
		INSERT INTO shortener.links (code, original_url)
		VALUES ($1, $2)
		RETURNING id, code, original_url, clicks, created_at, updated_at`

	created, err := scanLink(r.db.QueryRow(ctx, query, link.Code, link.OriginalURL))
	if err != nil {
		return entity.Link{}, mapError(err)
	}

	return created, nil
}

func (r *LinkRepository) GetByID(ctx context.Context, id string) (entity.Link, error) {
	const query = `
		SELECT id, code, original_url, clicks, created_at, updated_at
		FROM shortener.links
		WHERE id = $1`

	link, err := scanLink(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return entity.Link{}, mapError(err)
	}

	return link, nil
}

func (r *LinkRepository) List(ctx context.Context, limit, offset int) ([]entity.Link, int, error) {
	const (
		listQuery = `
			SELECT id, code, original_url, clicks, created_at, updated_at
			FROM shortener.links
			ORDER BY created_at DESC, id
			LIMIT $1 OFFSET $2`
		countQuery = `SELECT COUNT(*) FROM shortener.links`
	)

	var total int
	if err := r.db.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, listQuery, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	links, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (entity.Link, error) {
		return scanLink(row)
	})
	if err != nil {
		return nil, 0, err
	}

	return links, total, nil
}

func (r *LinkRepository) Update(ctx context.Context, id string, upd entity.LinkUpdate) (entity.Link, error) {
	const query = `
		UPDATE shortener.links
		SET original_url = COALESCE($2::text, original_url),
		    code         = COALESCE($3::text, code),
		    updated_at   = NOW()
		WHERE id = $1
		RETURNING id, code, original_url, clicks, created_at, updated_at`

	updated, err := scanLink(r.db.QueryRow(ctx, query, id, upd.OriginalURL, upd.Code))
	if err != nil {
		return entity.Link{}, mapError(err)
	}

	return updated, nil
}

func (r *LinkRepository) Delete(ctx context.Context, id string) error {
	const query = `DELETE FROM shortener.links WHERE id = $1`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return mapError(err)
	}

	if tag.RowsAffected() == 0 {
		return entity.ErrLinkNotFound
	}

	return nil
}

func (r *LinkRepository) Resolve(ctx context.Context, code string) (string, error) {
	const query = `
		UPDATE shortener.links
		SET clicks = clicks + 1
		WHERE code = $1
		RETURNING original_url`

	var originalURL string
	if err := r.db.QueryRow(ctx, query, code).Scan(&originalURL); err != nil {
		return "", mapError(err)
	}

	return originalURL, nil
}

func scanLink(row pgx.Row) (entity.Link, error) {
	var link entity.Link
	err := row.Scan(&link.ID, &link.Code, &link.OriginalURL, &link.Clicks, &link.CreatedAt, &link.UpdatedAt)

	return link, err
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ErrLinkNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == codeUniqueConstraint:
			return entity.ErrCodeAlreadyExists
		case pgErr.Code == pgInvalidTextRepresent:
			return entity.ErrLinkNotFound
		}
	}

	return err
}
