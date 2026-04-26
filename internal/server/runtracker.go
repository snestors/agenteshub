package server

import "sync"

// RunTracker keeps an in-memory tally of turns that are currently in flight,
// split by kind. The DB is the source of truth for mini-agent runs (`agent_runs.status='running'`),
// but main-agent and project-session turns are not persisted as runs — this
// tracker fills that gap so the UI can show "engine busy" indicators.
type RunTracker struct {
	mu     sync.Mutex
	counts map[string]int // 'main' | 'project'
}

func NewRunTracker() *RunTracker {
	return &RunTracker{counts: map[string]int{}}
}

// Inc bumps the counter for kind. Inc/Dec must be paired (typically with defer).
func (r *RunTracker) Inc(kind string) {
	r.mu.Lock()
	r.counts[kind]++
	r.mu.Unlock()
}

func (r *RunTracker) Dec(kind string) {
	r.mu.Lock()
	if r.counts[kind] > 0 {
		r.counts[kind]--
	}
	r.mu.Unlock()
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
