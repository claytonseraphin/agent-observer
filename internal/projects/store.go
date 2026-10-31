package projects

import (
	"context"
	"errors"
)

type ProjectStore interface {
	CreateProject(ctx context.Context, project Project) (Project, error)
}

var (
	ErrProjectNotFound  = errors.New("project not found")
	ErrAgentNotFound    = errors.New("agent not found")
	ErrNameTaken        = errors.New("project name already taken")
	ErrAgentExists      = errors.New("agent already exists in this project")
	ErrDefaultProtected = errors.New("the default project cannot be deleted")
)
