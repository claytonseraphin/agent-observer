package grouping

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// What the proxy remembers about one client address.
type clientActivity struct {
	id              string    // the group this client's calls are currently filed under
	runningCalls    int       // calls forwarded and not yet answered
	lastCallEndedAt time.Time // when the most recent call finished
}

// Decides which group ID an incoming call gets, using what it remembers per client.
type GroupAssigner struct {
	mu       sync.Mutex
	idleGap  time.Duration              // how long a group stays open after its last call ends
	byClient map[string]*clientActivity // client address -> that client's activity
}

func NewGroupAssigner(idleGap time.Duration) *GroupAssigner {
	return &GroupAssigner{
		idleGap:  idleGap,
		byClient: map[string]*clientActivity{},
	}
}

func (g *GroupAssigner) Begin(source string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	activity, ok := g.byClient[source]
	if !ok || (activity.runningCalls == 0 && time.Since(activity.lastCallEndedAt) > g.idleGap) {
		activity = &clientActivity{id: uuid.NewString()}
		g.byClient[source] = activity
	}
	activity.runningCalls++
	return activity.id
}

func (g *GroupAssigner) End(source string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if activity, ok := g.byClient[source]; ok {
		if activity.runningCalls > 0 {
			activity.runningCalls--
		}
		activity.lastCallEndedAt = time.Now()
	}
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
