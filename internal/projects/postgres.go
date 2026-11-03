package projects

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

var _ ProjectStore = (*PostgresStore)(nil)

func (s *PostgresStore) CreateProject(ctx context.Context, project Project) (Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Project{}, fmt.Errorf("transaction error: %w", err)
	}

	defer tx.Rollback(ctx)

	query := `
			INSERT INTO projects (id, name, key_hash, key_hint)
			VALUES($1, $2, NULLIF($3, ''), NULLIF($4, ''))
			RETURNING created_at
	`
	err = tx.QueryRow(
		ctx,
		query,
		project.ID,
		project.Name,
		project.KeyHash,
		project.KeyHint,
	).Scan(&project.CreatedAt)

	if err != nil {
		return Project{}, fmt.Errorf("inserting project: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Project{}, fmt.Errorf("committing project: %w", err)
	}
	project.CreatedAt = project.CreatedAt.UTC()

	return project, nil
}
