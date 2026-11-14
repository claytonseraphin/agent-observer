package api

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/claytonseraphin/agent-observer/internal/projects"
)

func (s *fakeStore) CreateProject(ctx context.Context, project projects.Project) (projects.Project, error) {
	if s.err != nil {
		return projects.Project{}, s.err
	}

	now := time.Now().UTC()

	project.CreatedAt = now

	for _, proj := range s.projects {
		if strings.EqualFold(proj.Name, project.Name) {
			return projects.Project{}, projects.ErrNameTaken
		}
	}

	var seen = make(map[string]bool)
	for i := range project.Agents {
		if seen[project.Agents[i].Name] {
			return projects.Project{}, projects.ErrAgentExists
		}
		seen[project.Agents[i].Name] = true
		project.Agents[i].CreatedAt = now
	}

	s.projects[project.ID] = project

	return project, nil
}

func (s *fakeStore) GetProject(ctx context.Context, id string) (projects.Project, error) {
	if s.err != nil {
		return projects.Project{}, s.err
	}

	project, ok := s.projects[id]
	if !ok {
		return projects.Project{}, projects.ErrProjectNotFound
	}

	return project, nil
}

func (s *fakeStore) ListProjects(ctx context.Context, limit int) ([]projects.ProjectSummary, error) {
	if s.err != nil {
		return []projects.ProjectSummary{}, s.err
	}

	summaries := []projects.ProjectSummary{}
	for _, project := range s.projects {
		summary := projects.ProjectSummary{
			ID:         project.ID,
			Name:       project.Name,
			KeyHint:    project.KeyHint,
			AgentCount: len(project.Agents),
			CreatedAt:  project.CreatedAt,
		}
		summaries = append(summaries, summary)
	}

	slices.SortFunc(summaries, func(a, b projects.ProjectSummary) int {
		c := a.CreatedAt.Compare(b.CreatedAt)
		if c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})

	if limit < len(summaries) {
		return summaries[:limit], nil
	}

	return summaries, nil
}

func (s *fakeStore) DeleteProject(ctx context.Context, id string) error {
	if s.err != nil {
		return s.err
	}
	if id == "default" {
		return projects.ErrDefaultProtected
	}
	_, ok := s.projects[id]
	if !ok {
		return projects.ErrProjectNotFound
	}

	delete(s.projects, id)

	return nil
}
