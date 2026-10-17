package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/claytonseraphin/agent-observer/internal/outcome"
	"github.com/claytonseraphin/agent-observer/internal/tracker"
)

type GroupSummary struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	AgentCount int        `json:"agent_count"`
	CallCount  int        `json:"call_count"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at"`
}

type Groups struct {
	Total  int            `json:"total"`
	Groups []GroupSummary `json:"groups"`
}

// groupTimes returns when the group started and, if every call has
// finished, when the last one ended (nil while anything is running).
func groupTimes(calls []tracker.CallView) (time.Time, *time.Time) {
	started := calls[0].StartedAt

	var latest time.Time
	for _, c := range calls {
		if c.Status == outcome.StatusProcessing || c.EndedAt == nil {
			return started, nil // still running: no end time yet
		}
		if c.EndedAt.After(latest) {
			latest = *c.EndedAt
		}
	}
	return started, &latest
}

func (h *handlers) getGroups(w http.ResponseWriter, r *http.Request) {
	views := h.tr.Snapshot()

	out := Groups{
		Total:  len(views),
		Groups: make([]GroupSummary, 0, len(views)),
	}

	for _, v := range views {
		started, ended := groupTimes(v.Calls)
		out.Groups = append(
			out.Groups, GroupSummary{
				ID:         v.ID,
				Status:     v.Status,
				AgentCount: v.AgentCount,
				CallCount:  v.CallCount,
				StartedAt:  started,
				EndedAt:    ended,
			})
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(out)
}

func (h *handlers) getGroup(w http.ResponseWriter, r *http.Request) {
	view, ok := h.tr.Group(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(view)
}

func (h *handlers) debug(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.tr.Snapshot())
}
