package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/claytonseraphin/agent-observer/internal/auth"
	"github.com/claytonseraphin/agent-observer/internal/projects"
)

type agentRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type createProjectRequest struct {
	Name   string         `json:"name"`
	Agents []agentRequest `json:"agents"`
}

type agentResponse struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	ProxyPath string    `json:"proxy_path"`
	CreatedAt time.Time `json:"created_at"`
}

type projectResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	KeyHint   string          `json:"key_hint"`
	CreatedAt time.Time       `json:"created_at"`
	Agents    []agentResponse `json:"agents"`
}

type createProjectResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Key       string          `json:"key"`
	KeyHint   string          `json:"key_hint"`
	CreatedAt time.Time       `json:"created_at"`
	Agents    []agentResponse `json:"agents"`
}

type projectSummaryResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	KeyHint    string    `json:"key_hint"`
	AgentCount int       `json:"agent_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type listProjectsResponse struct {
	Count    int                      `json:"total"`
	Projects []projectSummaryResponse `json:"projects"`
}

func (h *handlers) createProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req createProjectRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", fmt.Sprintf("body must not be larger than %d bytes", maxBytesErr.Limit))
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}

	name, err := projects.ValidateProjectName(req.Name)
	if err != nil {
		handleError(w, err)
		return
	}

	agents := make([]projects.Agent, 0, len(req.Agents))

	for _, agent := range req.Agents {
		err := projects.ValidateAgentName(agent.Name)
		if err != nil {
			handleError(w, err)
			return
		}

		err = projects.ValidateUpstreamURL(agent.URL)
		if err != nil {
			handleError(w, err)
			return
		}

		agents = append(agents, projects.Agent{Name: agent.Name, URL: agent.URL})

	}

	projectID := projects.NewProjectID()
	key := projects.NewKey()
	hashKey := projects.HashKey(key)
	keyHint := projects.KeyHint(key)

	reqProject := projects.Project{
		ID:      projectID,
		Name:    name,
		KeyHash: hashKey,
		KeyHint: keyHint,
		Agents:  agents,
	}
	response, err := h.store.CreateProject(r.Context(), reqProject)
	if err != nil {
		handleError(w, err)
		return
	}

	resp := createProjectResponse{
		ID:        response.ID,
		Name:      response.Name,
		Key:       key,
		KeyHint:   keyHint,
		CreatedAt: response.CreatedAt,
	}

	responseAgents := make([]agentResponse, 0, len(response.Agents))

	for _, agent := range response.Agents {
		responseAgents = append(responseAgents, agentResponse{
			Name:      agent.Name,
			URL:       agent.URL,
			ProxyPath: "/" + agent.Name + "/",
			CreatedAt: agent.CreatedAt,
		})
	}
	resp.Agents = responseAgents

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/projects/"+resp.ID)
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(resp)

}

func (h *handlers) getProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "request has no valid user")
		return
	}

	role := auth.RoleFor(user, id)
	if !role.Can(auth.Read) {
		handleError(
			w,
			projects.ErrProjectNotFound,
		)
		return
	}

	response, err := h.store.GetProject(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}

	getProjectResponse := projectResponse{
		ID:        response.ID,
		Name:      response.Name,
		KeyHint:   response.KeyHint,
		CreatedAt: response.CreatedAt,
	}

	agents := make([]agentResponse, 0, len(response.Agents))
	for _, agent := range response.Agents {
		agents = append(agents, agentResponse{
			Name:      agent.Name,
			URL:       agent.URL,
			ProxyPath: "/" + agent.Name + "/",
			CreatedAt: agent.CreatedAt,
		})
	}

	getProjectResponse.Agents = agents

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(getProjectResponse)

}

func (h *handlers) listProjects(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "request has no valid user")
		return
	}

	raw := r.URL.Query().Get("limit")
	limit, err := parseLimit(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a whole number between 1 and 200")
		return
	}

	response, err := h.store.ListProjects(r.Context(), limit)
	if err != nil {
		handleError(w, err)
		return
	}

	resp := listProjectsResponse{
		Count:    len(response),
		Projects: make([]projectSummaryResponse, 0, len(response)),
	}

	for _, summary := range response {
		resp.Projects = append(
			resp.Projects, projectSummaryResponse{
				ID:         summary.ID,
				Name:       summary.Name,
				KeyHint:    summary.KeyHint,
				AgentCount: summary.AgentCount,
				CreatedAt:  summary.CreatedAt,
			},
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resp)

}

func (h *handlers) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, found := auth.UserFromContext(r.Context())
	if !found {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "request has no valid user")
		return
	}

	role := auth.RoleFor(user, id)
	if !role.Can(auth.Delete) {
		handleError(w, projects.ErrProjectNotFound)
		return
	}

	if err := h.store.DeleteProject(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
