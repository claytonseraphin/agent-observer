package api

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	//"strconv"
	"testing"

	"github.com/claytonseraphin/agent-observer/internal/auth"
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
	s.lastLimit = limit
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

func TestCreateProjectErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(*fakeStore)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "malformed json",
			body:       `{"name": `,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "wrong type for name",
			body:       `{"name": 123}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "unknown field",
			body:       `{"name": "Research", "bogus": 1}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "body too large",
			body:       `{"name": "` + strings.Repeat("a", 1<<20) + `"}`,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantCode:   "body_too_large",
		},
		{
			name:       "empty project name",
			body:       `{"name": "   "}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_name",
		},
		{
			name:       "reserved agent name",
			body:       `{"name": "Research", "agents": [{"name": "api", "url": "http://localhost:8001"}]}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "reserved_agent_name",
		},
		{
			name:       "invalid upstream url",
			body:       `{"name": "Research", "agents": [{"name": "news", "url": "ftp://localhost:8001"}]}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_url",
		},
		{
			name: "name already taken",
			body: `{"name": "research"}`,
			setup: func(s *fakeStore) {
				s.projects["prj_seed00000001"] = projects.Project{ID: "prj_seed00000001", Name: "Research"}
			},
			wantStatus: http.StatusConflict,
			wantCode:   "name_taken",
		},
		{
			name: "duplicate agent names",
			body: `{"name": "Research", "agents": [
				{"name": "news", "url": "http://localhost:8001"},
				{"name": "news", "url": "http://localhost:8002"}
			]}`,
			wantStatus: http.StatusConflict,
			wantCode:   "agent_exists",
		},
		{
			name: "store failure",
			body: `{"name": "Research"}`,
			setup: func(s *fakeStore) {
				s.err = errors.New("connection refused: postgres:5432")
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			if tt.setup != nil {
				tt.setup(store)
			}
			before := len(store.projects)
			h := &handlers{store: store}

			req := httptest.NewRequest("POST", "/api/projects", strings.NewReader(tt.body))
			rr := httptest.NewRecorder()

			h.createProject(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if ct := rr.Result().Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("content type: got %q, want %q", ct, "application/json")
			}

			var got errorBody
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not an error body: %v; body: %s", err, rr.Body.String())
			}
			if got.Error.Code != tt.wantCode {
				t.Errorf("code: got %q, want %q", got.Error.Code, tt.wantCode)
			}
			if got.Error.Message == "" {
				t.Error("message is empty")
			}

			if strings.Contains(rr.Body.String(), "postgres") {
				t.Errorf("internal error text leaked to the client: %s", rr.Body.String())
			}

			if after := len(store.projects); after != before {
				t.Errorf("a failed request changed the store: %d projects before, %d after", before, after)
			}
		})
	}
}

func TestGetProject(t *testing.T) {
	store := newFakeStore()
	h := &handlers{store: store}
	ctx := context.Background()

	news := projects.Agent{Name: "news", URL: "http://host.docker.internal:8001"}
	finance := projects.Agent{Name: "financial", URL: "http://host.docker.internal:8002"}

	project := projects.Project{
		ID:      "prj_test1234",
		Name:    "Research Assistant",
		KeyHash: "hash1234",
		KeyHint: "1234",
	}

	project.Agents = append(project.Agents, news, finance)

	resp, err := store.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("error creating project, %v", err)
	}

	getProjectReq := httptest.NewRequest("GET", "/api/projects/"+resp.ID, nil)
	userCtx := auth.ContextWithUser(getProjectReq.Context(), auth.LocalUser())
	getProjectReq = getProjectReq.WithContext(userCtx)
	getProjectResponse := httptest.NewRecorder()

	getProjectReq.SetPathValue("id", resp.ID)
	h.getProject(getProjectResponse, getProjectReq)

	if getProjectResponse.Code != http.StatusOK {
		t.Fatalf("incorrect status code: got %v, want %v", getProjectResponse.Code, http.StatusOK)
	}
	contentType := getProjectResponse.Result().Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("incorrect Content-Type: got %q, want %q", contentType, "application/json")
	}

	getProjectBody := getProjectResponse.Body.String()
	var getProjectResp projectResponse
	if err := json.Unmarshal(getProjectResponse.Body.Bytes(), &getProjectResp); err != nil {
		t.Fatalf("error decoding response: %v; body: %v", err, getProjectBody)
	}

	if getProjectResp.ID != "prj_test1234" {
		t.Fatalf("incorrect id: got %v, want %v", getProjectResp.ID, "prj_test1234")
	}

	if getProjectResp.Name != "Research Assistant" {
		t.Fatalf("incorrect name: got %v, want %v", getProjectResp.Name, "Research Assistant")
	}

	if getProjectResp.KeyHint != "1234" {
		t.Fatalf("incorrect key hint: got %v, want %v", getProjectResp.KeyHint, "1234")
	}

	if getProjectResp.CreatedAt.IsZero() {
		t.Fatalf("stored project %q has no CreatedAt", getProjectResp.Name)
	}

	wantAgents := []agentResponse{
		{Name: "news", URL: "http://host.docker.internal:8001", ProxyPath: "/news/"},
		{Name: "financial", URL: "http://host.docker.internal:8002", ProxyPath: "/financial/"},
	}

	if len(getProjectResp.Agents) != len(wantAgents) {
		t.Fatalf("incorrect number of agents in response: got %d, want %d", len(getProjectResp.Agents), len(wantAgents))
	}

	for i, want := range wantAgents {
		got := getProjectResp.Agents[i]
		if got.Name != want.Name {
			t.Fatalf("incorrect agent name: got %v, want %v", got.Name, want.Name)
		}

		if got.URL != want.URL {
			t.Fatalf("incorrect agent url: got %v, want %v", got.URL, want.URL)
		}

		if got.ProxyPath != want.ProxyPath {
			t.Fatalf("incorrect agent proxy path: got %v, want %v", got.ProxyPath, want.ProxyPath)
		}

		if got.CreatedAt.IsZero() {
			t.Errorf("stored agent %q has no CreatedAt", got.Name)
		}
	}

	var raw map[string]any
	if err := json.Unmarshal(getProjectResponse.Body.Bytes(), &raw); err != nil {
		t.Fatalf("error decoding body as map: %v", err)
	}
	for _, field := range []string{"key", "key_hash"} {
		if _, found := raw[field]; found {
			t.Errorf("response must not contain %q", field)
		}
	}
}

func TestGetProjectErrors(t *testing.T) {
	seed := func(s *fakeStore) {
		s.projects["prj_test1234"] = projects.Project{ID: "prj_test1234", Name: "Research"}
	}

	tests := []struct {
		name       string
		id         string
		noUser     bool
		setup      func(*fakeStore)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "unknown id",
			id:         "prj_missing",
			wantStatus: http.StatusNotFound,
			wantCode:   "project_not_found",
		},
		{
			name:       "no user in context",
			id:         "prj_test1234",
			noUser:     true,
			setup:      seed,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name: "store failure",
			id:   "prj_test1234",
			setup: func(s *fakeStore) {
				seed(s)
				s.err = errors.New("connection refused: postgres:5432")
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			if tt.setup != nil {
				tt.setup(store)
			}
			h := &handlers{store: store}

			req := httptest.NewRequest("GET", "/api/projects/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			if !tt.noUser {
				req = req.WithContext(auth.ContextWithUser(req.Context(), auth.LocalUser()))
			}
			rr := httptest.NewRecorder()

			h.getProject(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if ct := rr.Result().Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("content type: got %q, want %q", ct, "application/json")
			}

			var got errorBody
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not an error body: %v; body: %s", err, rr.Body.String())
			}
			if got.Error.Code != tt.wantCode {
				t.Errorf("code: got %q, want %q", got.Error.Code, tt.wantCode)
			}
			if got.Error.Message == "" {
				t.Error("message is empty")
			}

			if strings.Contains(rr.Body.String(), "postgres") {
				t.Errorf("internal error text leaked to the client: %s", rr.Body.String())
			}
		})
	}
}

func TestListProjects(t *testing.T) {
	store := newFakeStore()
	h := &handlers{store: store}
	ctx := context.Background()

	news := projects.Agent{Name: "news", URL: "http://host.docker.internal:8001"}
	finance := projects.Agent{Name: "financial", URL: "http://host.docker.internal:8002"}

	projectOne := projects.Project{
		ID:      "prj_test1234",
		Name:    "Research Assistant",
		KeyHash: "hash1234",
		KeyHint: "1234",
	}

	projectOne.Agents = append(projectOne.Agents, news, finance)

	_, err := store.CreateProject(ctx, projectOne)
	if err != nil {
		t.Fatalf("error creating project, %v", err)
	}

	projectTwo := projects.Project{
		ID:      "prj_test5678",
		Name:    "Shopping Assistant",
		KeyHash: "hash5678",
		KeyHint: "5678",
	}

	_, err = store.CreateProject(ctx, projectTwo)
	if err != nil {
		t.Fatalf("error creating project, %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects?limit=60", nil)

	userCtx := auth.ContextWithUser(req.Context(), auth.LocalUser())
	req = req.WithContext(userCtx)

	rr := httptest.NewRecorder()

	h.listProjects(rr, req)

	if store.lastLimit != 60 {
		t.Errorf("limit passed to store: got %d, want %d", store.lastLimit, 60)
	}

	var summaries listProjectsResponse

	contentType := rr.Result().Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("incorrect content type: got %s, want %s", contentType, "application/json")
	}

	body := rr.Body.String()

	if rr.Code != http.StatusOK {
		t.Fatalf("incorrect status code: got %d, want %d", rr.Code, http.StatusOK)
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("error decoding the payload: %v, body %v", err, body)
	}

	var byID = make(map[string]projectSummaryResponse)

	if summaries.Count != 2 {
		t.Fatalf("incorrect number of project summaries listed: got %d, want %d", summaries.Count, 2)
	}
	for _, summary := range summaries.Projects {
		byID[summary.ID] = summary
	}

	gotOne, ok := byID["prj_test1234"]
	if !ok {
		t.Fatalf("project %s missing from project summaries", "prj_test1234")
	}

	if gotOne.Name != "Research Assistant" {
		t.Fatalf("incorrect project name listed for %s: got %s, want %s", "prj_test1234", gotOne.Name, "Research Assistant")
	}

	if gotOne.KeyHint != "1234" {
		t.Fatalf("incorrect project key hint listed for %s: got %s, want %s", "Research Assistant", gotOne.KeyHint, "1234")
	}

	if gotOne.CreatedAt.IsZero() {
		t.Fatalf("stored project summary %s has no created at", "Research Assistant")
	}

	if gotOne.AgentCount != 2 {
		t.Fatalf("incorrect number of agents for %s: got %d, want %d", "Research Assistant", gotOne.AgentCount, 2)
	}

	gotTwo, ok := byID["prj_test5678"]
	if !ok {
		t.Fatalf("project %s missing from project summaries", "prj_test5678")
	}

	if gotTwo.Name != "Shopping Assistant" {
		t.Fatalf("incorrect project name listed for %s: got %s, want %s", "prj_test5678", gotTwo.Name, "Shopping Assistant")
	}

	if gotTwo.KeyHint != "5678" {
		t.Fatalf("incorrect project key hint listed for %s: got %s, want %s", "Shopping Assistant", gotTwo.KeyHint, "5678")
	}

	if gotTwo.CreatedAt.IsZero() {
		t.Fatalf("stored project summary %s has no created at", "Shopping Assistant")
	}

	if gotTwo.AgentCount != 0 {
		t.Fatalf("incorrect number of agents for %s: got %d, want %d", gotTwo.Name, gotTwo.AgentCount, 0)
	}

	var raw map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("error decoding body as map: %v", err)
	}

	list, ok := raw["projects"].([]any)
	if !ok {
		t.Fatalf(`"projects" is missing or is not a list: %v`, raw["projects"])
	}

	for _, item := range list {
		project, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("project entry is not an object: %v", item)
		}
		for _, field := range []string{"key", "key_hash"} {
			if _, found := project[field]; found {
				t.Errorf("project must not contain %q", field)
			}
		}
	}
}

func TestListProjectsEmpty(t *testing.T) {
	store := newFakeStore()
	h := &handlers{store: store}

	req := httptest.NewRequest("GET", "/api/projects", nil)
	userCtx := auth.ContextWithUser(req.Context(), auth.LocalUser())
	req = req.WithContext(userCtx)

	rr := httptest.NewRecorder()

	h.listProjects(rr, req)

	contentType := rr.Result().Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("incorrect content type: got %s, want %s", contentType, "application/json")
	}

	if rr.Code != http.StatusOK {
		t.Fatalf("incorrect status code: got %d, want %d", rr.Code, http.StatusOK)
	}

	var response = listProjectsResponse{}
	body := rr.Body.String()
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("error decoding the payload: %v, body %v", err, body)
	}

	if response.Count != 0 {
		t.Fatalf("incorrect number of project summaries: got %d, want %d", response.Count, 0)
	}

	if response.Projects == nil {
		t.Errorf("projects should be an empty list, got null; body: %s", body)
	}
}

func TestListProjectsErrors(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		noUser     bool
		setup      func(*fakeStore)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "no user in context",
			url:        "/api/projects",
			noUser:     true,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name:       "no user wins over a bad limit",
			url:        "/api/projects?limit=abc",
			noUser:     true,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name:       "limit is not a number",
			url:        "/api/projects?limit=abc",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_limit",
		},
		{
			name:       "limit is zero",
			url:        "/api/projects?limit=0",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_limit",
		},
		{
			name:       "limit is negative",
			url:        "/api/projects?limit=-5",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_limit",
		},
		{
			name: "store failure",
			url:  "/api/projects",
			setup: func(s *fakeStore) {
				s.err = errors.New("connection refused: postgres:5432")
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			if tt.setup != nil {
				tt.setup(store)
			}
			h := &handlers{store: store}

			req := httptest.NewRequest("GET", tt.url, nil)
			if !tt.noUser {
				req = req.WithContext(auth.ContextWithUser(req.Context(), auth.LocalUser()))
			}
			rr := httptest.NewRecorder()

			h.listProjects(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if ct := rr.Result().Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("content type: got %q, want %q", ct, "application/json")
			}

			var got errorBody
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not an error body: %v; body: %s", err, rr.Body.String())
			}
			if got.Error.Code != tt.wantCode {
				t.Errorf("code: got %q, want %q", got.Error.Code, tt.wantCode)
			}
			if got.Error.Message == "" {
				t.Error("message is empty")
			}

			if strings.Contains(rr.Body.String(), "postgres") {
				t.Errorf("internal error text leaked to the client: %s", rr.Body.String())
			}

			if tt.wantCode == "invalid_limit" && store.lastLimit != 0 {
				t.Errorf("store was called with limit %d, but the request was rejected before it", store.lastLimit)
			}
		})
	}
}

func TestListProjectsLimit(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantLimit int
	}{
		{name: "no limit uses the default", url: "/api/projects", wantLimit: 50},
		{name: "limit is passed through", url: "/api/projects?limit=20", wantLimit: 20},
		{name: "limit above the max is clamped", url: "/api/projects?limit=500", wantLimit: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			h := &handlers{store: store}

			req := httptest.NewRequest("GET", tt.url, nil)
			req = req.WithContext(auth.ContextWithUser(req.Context(), auth.LocalUser()))
			rr := httptest.NewRecorder()

			h.listProjects(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
			}
			if store.lastLimit != tt.wantLimit {
				t.Errorf("limit passed to store: got %d, want %d", store.lastLimit, tt.wantLimit)
			}
		})
	}
}

func TestDeleteProject(t *testing.T) {
	store := newFakeStore()
	h := &handlers{store: store}
	ctx := context.Background()

	news := projects.Agent{Name: "news", URL: "http://host.docker.internal:8001"}
	finance := projects.Agent{Name: "financial", URL: "http://host.docker.internal:8002"}

	project := projects.Project{
		ID:      "prj_test1234",
		Name:    "Research Assistant",
		KeyHash: "hash1234",
		KeyHint: "1234",
	}

	project.Agents = append(project.Agents, news, finance)

	project, err := store.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("error creating project, %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/projects/"+project.ID, nil)
	userCtx := auth.ContextWithUser(req.Context(), auth.LocalUser())
	req = req.WithContext(userCtx)
	req.SetPathValue("id", project.ID)

	resp := httptest.NewRecorder()

	h.deleteProject(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("incorrect status code: got %d, want %d", resp.Code, http.StatusNoContent)
	}

	if resp.Body.Len() != 0 {
		t.Errorf("body should be empty, got: %s", resp.Body.String())
	}

	if _, found := store.projects[project.ID]; found {
		t.Errorf("project %s is still in the store after delete", project.ID)
	}

}

func TestDeleteProjectErrors(t *testing.T) {
	seed := func(s *fakeStore) {
		s.projects["prj_test1234"] = projects.Project{ID: "prj_test1234", Name: "Research"}
	}
	tests := []struct {
		name       string
		id         string
		noUser     bool
		setup      func(*fakeStore)
		wantStatus int
		wantCode   string
	}{
		{name: "unknown id", id: "prj_missing", wantStatus: http.StatusNotFound, wantCode: "project_not_found"},
		{name: "default project", id: "default", wantStatus: http.StatusConflict, wantCode: "default_protected"},
		{name: "no user", id: "prj_test1234", noUser: true, setup: seed, wantStatus: http.StatusUnauthorized, wantCode: "unauthenticated"},
		{
			name: "store failure",
			id:   "prj_test1234",
			setup: func(s *fakeStore) {
				seed(s)
				s.err = errors.New("connection refused: postgres:5432")
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			if tt.setup != nil {
				tt.setup(store)
			}
			before := len(store.projects)
			h := &handlers{store: store}

			req := httptest.NewRequest("DELETE", "/api/projects/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			if !tt.noUser {
				req = req.WithContext(auth.ContextWithUser(req.Context(), auth.LocalUser()))
			}
			rr := httptest.NewRecorder()

			h.deleteProject(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			var got errorBody
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not an error body: %v; body: %s", err, rr.Body.String())
			}
			if got.Error.Code != tt.wantCode {
				t.Errorf("code: got %q, want %q", got.Error.Code, tt.wantCode)
			}
			if strings.Contains(rr.Body.String(), "postgres") {
				t.Errorf("internal error text leaked to the client: %s", rr.Body.String())
			}
			if len(store.projects) != before {
				t.Errorf("a failed delete changed the store: %d projects before, %d after", before, len(store.projects))
			}
		})
	}
}
