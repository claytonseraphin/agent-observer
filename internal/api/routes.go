package api

import (
	"net/http"

	"github.com/claytonseraphin/agent-observer/internal/auth"
	"github.com/claytonseraphin/agent-observer/internal/projects"
	"github.com/claytonseraphin/agent-observer/internal/tracker"
)

type handlers struct {
	tr    *tracker.Tracker
	store projects.ProjectStore
}

func Register(mux *http.ServeMux, tr *tracker.Tracker, store projects.ProjectStore) {
	h := &handlers{tr: tr, store: store}

	protect := func(fn http.HandlerFunc) http.Handler {
		return auth.NoAuth(fn)
	}

	mux.HandleFunc("GET /api/groups", h.getGroups)
	mux.HandleFunc("GET /api/groups/{id}", h.getGroup)
	mux.HandleFunc("/debug/requests", h.debug)

	mux.Handle("POST /api/projects", protect(h.createProject))
	mux.Handle("GET /api/projects/{id}", protect(h.getProject))
	mux.Handle("GET /api/projects", protect(h.listProjects))
	mux.Handle("DELETE /api/projects/{id}", protect(h.deleteProject))
}
