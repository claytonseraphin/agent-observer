package projects

import (
	"context"
	"errors"
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
	return Project{}, errors.New("not implemented")
}
