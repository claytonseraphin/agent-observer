package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	Total    int                      `json:"total"`
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
	return
}

func (h *handlers) listProjects(w http.ResponseWriter, r *http.Request) {
	return
}

func (h *handlers) deleteProject(w http.ResponseWriter, r *http.Request) {
	return
}
