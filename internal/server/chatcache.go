package server

import (
	"context"
	"sync"
	"time"

	"lessmess/internal/opencode"
)

// The poll-path read collapse: every open chat view polls every ~1.2s,
// so N views of one session and every concurrent poll of any session
// all request the same global busy map. Under a slow service that
// self-inflicted multiplication is exactly the load to avoid. Two
// layers: a TTL + single-flight cache for ListActiveSessions (global,
// shared across sessions) and a per-session single-flight around the
// whole snapshot fan-out (concurrent polls of one session share one
// upstream transcript fetch). Sequential polls are unaffected — only
// genuinely concurrent work collapses.

// chatActiveTTL bounds how long a cached ListActiveSessions answer
// serves. At the client's ~1.2s poll cadence one second of busy-state
// staleness is invisible. Var, not const, so tests can shrink it.
var chatActiveTTL = time.Second

// activeCache memoizes ListActiveSessions for chatActiveTTL and
// collapses concurrent fetches into one. Successes are memoized;
// failures never are — the next poll retries upstream.
type activeCache struct {
	mu       sync.Mutex
	val      map[string]opencode.SessionActive
	at       time.Time
	inflight chan activeResult
}

type activeResult struct {
	val map[string]opencode.SessionActive
	err error
}

// get serves the cached value, joins the in-flight fetch, or starts it.
// A nil cache is inert (direct fetch) so hand-built Servers keep working.
func (c *activeCache) get(ctx context.Context, fetch func(context.Context) (map[string]opencode.SessionActive, error)) (map[string]opencode.SessionActive, error) {
	if c == nil {
		return fetch(ctx)
	}
	c.mu.Lock()
	if c.val != nil && time.Since(c.at) < chatActiveTTL {
		v := c.val
		c.mu.Unlock()
		return v, nil
	}
	if c.inflight != nil {
		ch := c.inflight
		c.mu.Unlock()
		select {
		case r := <-ch:
			return r.val, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ch := make(chan activeResult, 1)
	c.inflight = ch
	c.mu.Unlock()
	// The shared fetch runs on its own budget: no single requester's
	// cancellation kills it for the others waiting on it.
	fctx, cancel := context.WithTimeout(context.Background(), chatPollBudget)
	val, err := fetch(fctx)
	cancel()
	ch <- activeResult{val: val, err: err}
	c.mu.Lock()
	c.inflight = nil
	if err == nil {
		c.val, c.at = val, time.Now()
	}
	c.mu.Unlock()
	return val, err
}

// snapshotData is one poll's raw upstream reads: the transcript page
// plus the three auxiliary reads, each with its own error.
type snapshotData struct {
	page        opencode.MessagePage
	pageErr     error
	active      map[string]opencode.SessionActive
	activeErr   error
	permissions []opencode.PermissionRequest
	permErr     error
	forms       []opencode.Form
	formErr     error
}

// snapshotMemo holds the last successful latest-page fan-out per
// session so a failing transcript read can still render a stale (marked
// degraded) transcript instead of the UI's loading placeholder. History
// pages are never memoized.
type snapshotMemo struct {
	mu sync.Mutex
	m  map[string]memoEntry
}

type memoEntry struct {
	data snapshotData
	at   time.Time
}

// chatMemoTTL bounds how long a last-good transcript serves during
// outages. Var, not const, so tests can shrink it.
var chatMemoTTL = 10 * time.Minute

func (m *snapshotMemo) put(sessionID string, data snapshotData) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.m == nil {
		m.m = map[string]memoEntry{}
	}
	m.m[sessionID] = memoEntry{data: data, at: time.Now()}
	m.mu.Unlock()
}

// get returns the memoized data for session when it is fresh enough,
// with its store time for age logging.
func (m *snapshotMemo) get(sessionID string) (snapshotData, time.Time, bool) {
	if m == nil {
		return snapshotData{}, time.Time{}, false
	}
	m.mu.Lock()
	e, ok := m.m[sessionID]
	m.mu.Unlock()
	if !ok || time.Since(e.at) > chatMemoTTL {
		return snapshotData{}, time.Time{}, false
	}
	return e.data, e.at, true
}

// snapshotFlight collapses concurrent snapshot polls of the same
// session and cursor into one upstream fan-out: N open views of one
// session cost one transcript fetch, not N. Reads are idempotent, so
// sharing the result is safe.
type snapshotFlight struct {
	mu    sync.Mutex
	calls map[string]*snapshotCall
}

type snapshotCall struct {
	done chan struct{}
	data snapshotData
}

// do returns the shared snapshotData for key, running fetch exactly
// once for all concurrent callers. ok is false only when the caller's
// ctx died while waiting — the caller surfaces that as a poll error.
// A nil flight is inert (direct fetch) so hand-built Servers keep
// working.
func (f *snapshotFlight) do(key string, waitCtx context.Context, fetch func(context.Context) snapshotData) (snapshotData, bool) {
	if f == nil {
		return fetch(waitCtx), true
	}
	f.mu.Lock()
	if c, ok := f.calls[key]; ok {
		f.mu.Unlock()
		select {
		case <-c.done:
			return c.data, true
		case <-waitCtx.Done():
			return snapshotData{}, false
		}
	}
	c := &snapshotCall{done: make(chan struct{})}
	f.calls[key] = c
	f.mu.Unlock()
	// Detached budget: one requester's disconnect must not cancel the
	// shared fetch the others are waiting on.
	ctx, cancel := context.WithTimeout(context.Background(), chatPollBudget)
	c.data = fetch(ctx)
	cancel()
	close(c.done)
	f.mu.Lock()
	delete(f.calls, key)
	f.mu.Unlock()
	return c.data, true
}
