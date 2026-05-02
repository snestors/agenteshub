package server

import (
	"context"
	"sync"
)

// RunTracker keeps an in-memory tally of turns that are currently in flight,
// split by kind. The DB is the source of truth for mini-agent runs (`agent_runs.status='running'`),
// but main-agent and project-session turns are not persisted as runs — this
// tracker fills that gap so the UI can show "engine busy" indicators.
//
// It also owns a registry of cancel functions keyed by `<scope>:<id>` so any
// caller (HTTP handler, MCP tool, slash command) can cancel an in-flight run
// regardless of where it was kicked off from. Project chats already had this
// via projectCancels; the registry generalises the pattern across main and
// agent scopes.
type RunTracker struct {
	mu             sync.Mutex
	counts         map[string]int                     // 'main' | 'project' | 'agent'
	cancels        map[string]context.CancelFunc      // 'scope:id' → cancel
	pendingRestart bool
	onZero         func() // fired once when restart is pending and total hits 0
}

func NewRunTracker() *RunTracker {
	return &RunTracker{
		counts:  map[string]int{},
		cancels: map[string]context.CancelFunc{},
	}
}

// RegisterCancel stores the cancel func for an in-flight run. Pair with
// UnregisterCancel via defer when the run completes naturally; Cancel() also
// removes the entry. Empty scope or id is a no-op.
func (r *RunTracker) RegisterCancel(scope, id string, cancel context.CancelFunc) {
	if scope == "" || id == "" || cancel == nil {
		return
	}
	r.mu.Lock()
	r.cancels[scope+":"+id] = cancel
	r.mu.Unlock()
}

// UnregisterCancel forgets the cancel for a key without firing it. Safe to call
// even if the key is not present (e.g. another caller already cancelled).
func (r *RunTracker) UnregisterCancel(scope, id string) {
	if scope == "" || id == "" {
		return
	}
	r.mu.Lock()
	delete(r.cancels, scope+":"+id)
	r.mu.Unlock()
}

// Cancel fires the registered cancel func for `scope:id` and removes the
// entry. Returns true when something was cancelled, false when no run was
// registered under that key.
func (r *RunTracker) Cancel(scope, id string) bool {
	if scope == "" || id == "" {
		return false
	}
	key := scope + ":" + id
	r.mu.Lock()
	cancel, ok := r.cancels[key]
	if ok {
		delete(r.cancels, key)
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// HasCancel reports whether there's a registered cancel for `scope:id`.
func (r *RunTracker) HasCancel(scope, id string) bool {
	if scope == "" || id == "" {
		return false
	}
	r.mu.Lock()
	_, ok := r.cancels[scope+":"+id]
	r.mu.Unlock()
	return ok
}

// Inc bumps the counter for kind. Inc/Dec must be paired (typically with defer).
func (r *RunTracker) Inc(kind string) {
	r.mu.Lock()
	r.counts[kind]++
	r.mu.Unlock()
}

func (r *RunTracker) Dec(kind string) {
	var cb func()
	r.mu.Lock()
	if r.counts[kind] > 0 {
		r.counts[kind]--
	}
	if r.pendingRestart && r.total() == 0 {
		cb = r.onZero
		r.pendingRestart = false
		r.onZero = nil
	}
	r.mu.Unlock()
	if cb != nil {
		go cb()
	}
}

// ScheduleRestart marks the tracker so that cb is called once all in-flight
// runs complete. If total is already zero, cb is called immediately.
// Safe to call from any goroutine, including an active run — no deadlock.
func (r *RunTracker) ScheduleRestart(cb func()) {
	r.mu.Lock()
	r.pendingRestart = true
	r.onZero = cb
	fire := r.total() == 0
	if fire {
		r.pendingRestart = false
		r.onZero = nil
	}
	r.mu.Unlock()
	if fire {
		go cb()
	}
}

// PendingRestart reports whether a restart has been scheduled.
func (r *RunTracker) PendingRestart() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pendingRestart
}

// total returns the sum of all counts. Must be called with r.mu held.
func (r *RunTracker) total() int {
	n := 0
	for _, v := range r.counts {
		n += v
	}
	return n
}

// Snapshot returns a copy of the current counts. Safe for concurrent reads.
func (r *RunTracker) Snapshot() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.counts))
	for k, v := range r.counts {
		out[k] = v
	}
	return out
}

// Count returns the in-flight count for a single kind (0 if missing).
func (r *RunTracker) Count(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[kind]
}
