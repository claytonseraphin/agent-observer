package main

import (
  "time"
)

// A safe rule has two conditions:
//
// No call is still processing. If any call is running, keep the group.
// The latest EndedAt is older than your retention time.
// Measure from when the group last did something, not when it started.

func startCleanup(every, maxAge time.Duration) {
  go func() {
    ticker := time.NewTicker(every)
    defer ticker.Stop()
    for range ticker.C {
      tracker.Cleanup(maxAge)
      assigner.Cleanup(maxAge)
    }
  }()
}

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
    if call.Status == StatusProcessing {
      return false
    }
    if call.EndedAt.After(latest) {
      latest = call.EndedAt
    }
  }

  return latest.Before(cutoff)
}


// GroupAssigner Cleanup
// Cleanup removes clients that have no running calls and have been
// quiet for longer than maxIdle.
func (g *GroupAssigner) Cleanup(maxIdle time.Duration) {
  g.mu.Lock()
  defer g.mu.Unlock()

  // Never forget a client sooner than the idle gap, or the gap logic breaks.
  if maxIdle < g.idleGap {
    maxIdle = g.idleGap
  }
  cutoff := time.Now().Add(-maxIdle)

  for address, activity := range g.byClient {
    if activity.runningCalls > 0 {
      continue // still has a call in flight
    }
    if activity.lastCallEndedAt.Before(cutoff) {
      delete(g.byClient, address)
    }
  }
  //log.Printf("assigner clients=%d", len(g.byClient))
}
