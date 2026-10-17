package api

import (
	"net/http"

	"github.com/claytonseraphin/agent-observer/internal/tracker"
)

type handlers struct {
	tr *tracker.Tracker
}

func Register(mux *http.ServeMux, tr *tracker.Tracker) {
	h := &handlers{tr: tr}
	mux.HandleFunc("GET /api/groups", h.getGroups)
	mux.HandleFunc("GET /api/groups/{id}", h.getGroup)
	mux.HandleFunc("/debug/requests", h.debug)
}
