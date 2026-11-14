package api

import (
	"github.com/claytonseraphin/agent-observer/internal/projects"
)

type fakeStore struct {
	projects map[string]projects.Project
	err      error
}

func newFakeStore() *fakeStore {
	return &fakeStore{projects: map[string]projects.Project{}}
}

var _ projects.ProjectStore = (*fakeStore)(nil)
