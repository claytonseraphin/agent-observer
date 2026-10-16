package tracker

import (
  "time"
  "sync"

  "github.com/claytonseraphin/agent-observer/internal/outcome"
)

type CallRecord struct {
  Agent        string
  Kind         string
  StartedAt    time.Time
  EndedAt      time.Time
  Status       string   // processing, success, failed
  Error        string
}

type GroupRecord struct {
  Origin       string   // Client or inferred
  Calls        []*CallRecord
}

type Tracker struct {
  mu          sync.Mutex
  groups     map[string]*GroupRecord
}


func (t *Tracker) StartCall(groupID, origin, agent, kind string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	group, ok := t.groups[groupID]
	if !ok {
		group = &GroupRecord{Origin: origin}
		t.groups[groupID] = group
	}

	call := &CallRecord{
		Agent:     agent,
    Kind:      kind,
		StartedAt: time.Now(),
		Status:    outcome.StatusProcessing,
	}
	group.Calls = append(group.Calls, call)

	return len(group.Calls) - 1
}

func (t *Tracker) FinishCall(groupID string, callIndex int, result outcome.CallOutcome) {
	t.mu.Lock()
	defer t.mu.Unlock()

	g, ok := t.groups[groupID]
	if !ok || callIndex < 0 || callIndex >= len(g.Calls) {
		return
	}
	call := g.Calls[callIndex]
	if call.Status != outcome.StatusProcessing {
		return // already finished, keep the first outcome
	}
	call.EndedAt = time.Now()
	call.Status = result.Status
	call.Error = result.Error
}

func NewTracker() *Tracker {
	return &Tracker{groups: map[string]*GroupRecord{}}
}
