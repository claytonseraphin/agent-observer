package projects

import (
	"context"
	"errors"
)

type ProjectStore interface {
	CreateProject(ctx context.Context, project Project) (Project, error)
	GetProject(ctx context.Context, id string) (Project, error)
	DeleteProject(ctx context.Context, id string) error
}

var (
	ErrProjectNotFound  = errors.New("project not found")
	ErrAgentNotFound    = errors.New("agent not found")
	ErrNameTaken        = errors.New("project name already taken")
	ErrAgentExists      = errors.New("agent already exists in this project")
	ErrDefaultProtected = errors.New("the default project cannot be deleted")
)
