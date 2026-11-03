package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const projectNameIndex = "projects_name_lower_idx"

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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == projectNameIndex {
			return Project{}, ErrNameTaken
		}
		return Project{}, fmt.Errorf("inserting project: %w", err)
	}

	// add agents in create project
	agentQuery := `
			INSERT INTO project_agents (project_id, name, upstream_url)
			VALUES($1, $2, $3)
			RETURNING created_at
	`

	for i := range project.Agents {
		err = tx.QueryRow(
			ctx,
			agentQuery,
			project.ID,
			project.Agents[i].Name,
			project.Agents[i].URL,
		).Scan(&project.Agents[i].CreatedAt)

		if err != nil {
			return Project{}, fmt.Errorf("inserting agent %q: %w", project.Agents[i].Name, err)
		}
		project.Agents[i].CreatedAt = project.Agents[i].CreatedAt.UTC()
	}

	if err := tx.Commit(ctx); err != nil {
		return Project{}, fmt.Errorf("committing project: %w", err)
	}
	project.CreatedAt = project.CreatedAt.UTC()

	return project, nil
}
