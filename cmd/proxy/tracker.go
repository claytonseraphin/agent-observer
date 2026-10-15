package main

import (
  "time"
  "sync"
  "net/url"
  "log"
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
		Status:    StatusProcessing,
	}
	group.Calls = append(group.Calls, call)

	return len(group.Calls) - 1
}

func (t *Tracker) FinishCall(groupID string, callIndex int, outcome CallOutcome) {
	t.mu.Lock()
	defer t.mu.Unlock()

	g, ok := t.groups[groupID]
	if !ok || callIndex < 0 || callIndex >= len(g.Calls) {
		return
	}
	call := g.Calls[callIndex]
	if call.Status != StatusProcessing {
		return // already finished, keep the first outcome
	}
	call.EndedAt = time.Now()
	call.Status = outcome.Status
	call.Error = outcome.Error
}

func NewTracker() *Tracker {
	return &Tracker{groups: map[string]*GroupRecord{}}
}

var tracker = NewTracker()

var assigner = NewGroupAssigner(10 * time.Second)

func mustParse(s string) *url.URL {
  u, err := url.Parse(s)
  if err != nil {
    log.Fatal(err)
  }
  return u
}
