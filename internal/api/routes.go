package api

import (
	"net/http"

	"github.com/claytonseraphin/agent-observer/internal/projects"
	"github.com/claytonseraphin/agent-observer/internal/tracker"
)

type handlers struct {
	tr    *tracker.Tracker
	store projects.ProjectStore
}

func Register(mux *http.ServeMux, tr *tracker.Tracker, store projects.ProjectStore) {
	h := &handlers{tr: tr, store: store}
	mux.HandleFunc("GET /api/groups", h.getGroups)
	mux.HandleFunc("GET /api/groups/{id}", h.getGroup)
	mux.HandleFunc("/debug/requests", h.debug)

	mux.HandleFunc("POST /api/projects", h.createProject)
	mux.HandleFunc("GET /api/projects/{id}", h.getProject)
	mux.HandleFunc("GET /api/projects", h.listProjects)
	mux.HandleFunc("DELETE /api/projects/{id}", h.deleteProject)
}
