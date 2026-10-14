package main


import (
  //"fmt"
  "time"
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


func (t *Tracker) Snapshot() []GroupView {
  t.mu.Lock()
  defer t.mu.Unlock()

  views := make([]GroupView, 0, len(t.groups))
  for id, rec := range t.groups {
    calls := make([]CallView, 0, len(rec.Calls))

    agents := map[string]bool{}
    anyProcessing, anyFailed := false, false

    for _, c := range rec.Calls {
      agents[c.Agent] = true
      if c.Status == StatusProcessing {
        anyProcessing = true
      }
      if c.Status == StatusFailed {
        anyFailed = true
      }

      // end := c.EndedAt
      // if c.Status == StatusProcessing {
      //   end = time.Now() // still running, so measure up to now
      // }
      cv := CallView{
        Agent:      c.Agent,
        Kind:       c.Kind,
        Status:     c.Status,
        StartedAt:  c.StartedAt,
        //DurationMs: end.Sub(c.StartedAt).Milliseconds(),
        Error:      c.Error,
      }

      if c.Status == StatusProcessing {
      	cv.DurationMs = time.Since(c.StartedAt).Milliseconds()
      } else {
      	ended := c.EndedAt // copy, so the pointer doesn't aim at shared state
      	cv.EndedAt = &ended
      	cv.DurationMs = ended.Sub(c.StartedAt).Milliseconds()
      }
      calls = append(calls, cv)

    }

    status := StatusSuccess
    switch {
    case anyProcessing:
      status = StatusProcessing
    case anyFailed:
      status = StatusFailed
    }

    views = append(views, GroupView{
      ID:           id,
      Origin:       rec.Origin,
      Status:       status,
      AgentCount:   len(agents),
      CallCount:    len(rec.Calls),
      Calls:        calls,
    })
  }
  return views
}
