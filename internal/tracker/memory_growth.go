package tracker

import (
	"time"

	"github.com/claytonseraphin/agent-observer/internal/outcome"
)

func (t *Tracker) Cleanup(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, g := range t.groups {
		if groupIsOld(g, cutoff) {
			delete(t.groups, id)
		}
	}
}

func groupIsOld(g *GroupRecord, cutoff time.Time) bool {
	var latest time.Time

	for _, call := range g.Calls {
		if call.Status == outcome.StatusProcessing {
			return false
		}
		if call.EndedAt.After(latest) {
			latest = call.EndedAt
		}
	}

	return latest.Before(cutoff)
}
