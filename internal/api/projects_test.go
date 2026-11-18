package api

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

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

func TestCreateProject(t *testing.T) {
	store := newFakeStore()
	h := &handlers{store: store}

	payload := `{
	    "name": "Research Assistant",
	    "agents": [
	        {"name": "news", "url": "http://host.docker.internal:8001"},
	        {"name": "financial", "url": "http://host.docker.internal:8002"}
	    ]
	}`

	req := httptest.NewRequest("POST", "/api/projects", strings.NewReader(payload))
	rr := httptest.NewRecorder()

	h.createProject(rr, req)

	body := rr.Body.String()
	t.Logf("status %d, body: %s", rr.Code, body)

	if rr.Code != http.StatusCreated {
		t.Fatalf("incorrect status code: got %d, want %d; body: %s", rr.Code, http.StatusCreated, body)
	}

	contentType := rr.Result().Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("incorrect content type: got %q, want %q", contentType, "application/json")
	}

	if strings.Contains(body, "key_hash") {
		t.Errorf("response must not contain the key hash; body: %s", body)
	}

	var resp createProjectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("error decoding response: %v; body: %s", err, body)
	}

	var projectIDRegex = regexp.MustCompile(`^prj_[a-z0-9]{12}$`)
	if !projectIDRegex.MatchString(resp.ID) {
		t.Fatalf("incorrect id pattern: got %q", resp.ID)
	}

	var projectKeyRegex = regexp.MustCompile(`^obs_[A-Za-z0-9_-]{43}$`)
	if !projectKeyRegex.MatchString(resp.Key) {
		t.Fatalf("incorrect key pattern: got %q", resp.Key)
	}

	wantLocation := "/api/projects/" + resp.ID
	if got := rr.Result().Header.Get("Location"); got != wantLocation {
		t.Errorf("incorrect location: got %q, want %q", got, wantLocation)
	}

	if resp.Name != "Research Assistant" {
		t.Errorf("incorrect name: got %q, want %q", resp.Name, "Research Assistant")
	}

	wantAgents := []agentResponse{
		{Name: "news", URL: "http://host.docker.internal:8001", ProxyPath: "/news/"},
		{Name: "financial", URL: "http://host.docker.internal:8002", ProxyPath: "/financial/"},
	}
	if len(resp.Agents) != len(wantAgents) {
		t.Fatalf("incorrect number of agents in response: got %d, want %d", len(resp.Agents), len(wantAgents))
	}
	for i, want := range wantAgents {
		got := resp.Agents[i]
		if got.Name != want.Name {
			t.Errorf("agent %d name: got %q, want %q", i, got.Name, want.Name)
		}
		if got.URL != want.URL {
			t.Errorf("agent %d url: got %q, want %q", i, got.URL, want.URL)
		}
		if got.ProxyPath != want.ProxyPath {
			t.Errorf("agent %d proxy path: got %q, want %q", i, got.ProxyPath, want.ProxyPath)
		}
	}

	stored, ok := store.projects[resp.ID]
	if !ok {
		t.Fatalf("project %q was not stored", resp.ID)
	}

	if stored.Name != "Research Assistant" {
		t.Errorf("stored name: got %q, want %q", stored.Name, "Research Assistant")
	}

	if wantHash := projects.HashKey(resp.Key); stored.KeyHash != wantHash {
		t.Errorf("stored hash does not match the hash of the returned key: got %q, want %q", stored.KeyHash, wantHash)
	}

	if stored.KeyHash == resp.Key {
		t.Errorf("the plain key was stored instead of its hash")
	}

	if stored.KeyHint != resp.KeyHint {
		t.Errorf("stored key hint: got %q, want %q", stored.KeyHint, resp.KeyHint)
	}

	if len(stored.Agents) != 2 {
		t.Fatalf("incorrect number of stored agents: got %d, want %d", len(stored.Agents), 2)
	}

	for _, agent := range stored.Agents {
		if agent.CreatedAt.IsZero() {
			t.Errorf("stored agent %q has no CreatedAt", agent.Name)
		}
	}
}
