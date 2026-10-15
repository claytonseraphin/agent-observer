package main


import (
  //"fmt"
  "time"
  "sort"
)


type CallView struct {
  Agent         string       `json:"agent"`
  Kind         string        `json:"kind"`
  Status        string       `json:"status"`
  StartedAt     time.Time    `json:"started_at"`
  EndedAt       *time.Time   `json:"ended_at,omitempty"`
  DurationMs    int64        `json:"duration_ms"`
  Error         string       `json:"error,omitempty"`
}


type GroupView struct {
  ID            string       `json:"id"`
  Origin        string       `json:"origin"`
  Status        string       `json:"status"`
  AgentCount    int          `json:"agent_count"`
  CallCount     int          `json:"call_count"`
  Calls         []CallView   `json:"calls"`
}


// buildView turns one group record into its JSON view.
// It does not lock: the caller must already hold t.mu.
func buildView(id string, g *GroupRecord) GroupView {
  view := GroupView{
    ID:        id,
    Origin:    g.Origin,
    CallCount: len(g.Calls),
    Calls:     make([]CallView, 0, len(g.Calls)),
  }

  agents := map[string]bool{}
  anyProcessing, anyFailed := false, false

  for _, c := range g.Calls {
    agents[c.Agent] = true

    cv := CallView{
      Agent:     c.Agent,
      Kind:      c.Kind,
      Status:    c.Status,
      StartedAt: c.StartedAt,
      Error:     c.Error,
    }
    if c.Status == StatusProcessing {
      anyProcessing = true
      cv.DurationMs = time.Since(c.StartedAt).Milliseconds()
    } else {
      ended := c.EndedAt // copy, so the pointer doesn't aim at shared state
      cv.EndedAt = &ended
      cv.DurationMs = ended.Sub(c.StartedAt).Milliseconds()
    }
    if c.Status == StatusFailed {
      anyFailed = true
    }
    view.Calls = append(view.Calls, cv)
  }

  view.AgentCount = len(agents)
  switch {
  case anyProcessing:
    view.Status = StatusProcessing
  case anyFailed:
    view.Status = StatusFailed
  default:
    view.Status = StatusSuccess
  }
  return view
}


func (t *Tracker) Snapshot() []GroupView {
  t.mu.Lock()
  defer t.mu.Unlock()

  views := make([]GroupView, 0, len(t.groups))
  for id, g := range t.groups {
    views = append(views, buildView(id, g))
  }
  sort.Slice(views, func(i, j int) bool {
    return views[i].Calls[0].StartedAt.After(views[j].Calls[0].StartedAt)
  })
  return views
}



func (t *Tracker) Group(id string) (GroupView, bool) {
  t.mu.Lock()
  defer t.mu.Unlock()

  g, ok := t.groups[id]
  if !ok {
    return GroupView{}, false
  }
  return buildView(id, g), true
}
