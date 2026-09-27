package server

import (
	"log/slog"
	"strings"
	"sync"
)

// Compaction re-prime: when a session's context is compacted, the base
// prompt (binding facts) and the state snapshot are summarized away.
// Capabilities need no restore — the skill listing survives compaction
// by construction — but the personalized parts do not, so the next
// prompt to a bound session carries a compact re-prime: the binding
// line, a fresh state snapshot, and a pointer back to the skills.
//
// Detection is deliberately transcript-based: lessmess marks the
// sessions whose compactions it queued (the board's Compact action),
// and the chat snapshot walk observes completed compaction messages —
// which also covers service-side auto-compaction, since every Chat view
// polls snapshots while visible. No event subscription: the opencode
// event stream is volatile by contract, and the transcript read is the
// same stable path the view already uses.

// compactionWatch is the per-process memory of which compactions have
// been seen and which have been compensated. Memory-only on purpose: a
// restart loses the markers, and the worst case is one uncompensated
// compaction after a restart.
type compactionWatch struct {
	mu       sync.Mutex
	pending  map[string]bool   // session → lessmess queued a compaction
	lastSeen map[string]string // session → newest completed compaction msg id
	primed   map[string]string // session → newest compensated compaction msg id
}

func newCompactionWatch() *compactionWatch {
	return &compactionWatch{
		pending:  map[string]bool{},
		lastSeen: map[string]string{},
		primed:   map[string]string{},
	}
}

// markPending records a compaction lessmess itself queued.
func (w *compactionWatch) markPending(sessionID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending[sessionID] = true
}

// observe records the newest completed compaction seen in a session's
// transcript.
func (w *compactionWatch) observe(sessionID, msgID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lastSeen[sessionID] == msgID {
		return
	}
	w.lastSeen[sessionID] = msgID
}

// needsReprime reports whether the session has a compaction that no
// wrap has compensated yet.
func (w *compactionWatch) needsReprime(sessionID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending[sessionID] {
		return true
	}
	seen, has := w.lastSeen[sessionID]
	return has && seen != "" && seen != w.primed[sessionID]
}

// consume records that the session's current compaction state has been
// compensated.
func (w *compactionWatch) consume(sessionID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.pending, sessionID)
	if seen := w.lastSeen[sessionID]; seen != "" {
		w.primed[sessionID] = seen
	}
}

// maybeReprime returns the prompt text to send: either the caller's text
// unchanged, or the re-prime block followed by it. Unbound sessions are
// never wrapped (a free chat has no binding to restore), and any
// lookup or read failure fails open — a user prompt is never blocked by
// this path.
func (s *Server) maybeReprime(sessionID, text string) string {
	if s.compacts == nil || s.mapErr != nil || !s.compacts.needsReprime(sessionID) {
		return text
	}
	changeID, entry, bound := s.sessions.entry(sessionID)
	if !bound {
		return text
	}
	var b strings.Builder
	if entry.Task != "" {
		b.WriteString("Context restoration: the conversation above was compacted. This session is bound to task " + entry.Task + " of change " + changeID + ".")
	} else {
		b.WriteString("Context restoration: the conversation above was compacted. This session is bound to change " + changeID + ".")
	}
	snapshot, err := s.st.LedgerFile(changeID)
	if err != nil {
		slog.Warn("reprime skipped: ledger unreadable", "change", changeID, "err", err)
		return text
	}
	b.WriteString("\n\nCurrent state (tool-injected; authoritative):\n\n")
	b.WriteString(snapshot)
	b.WriteString("\n\nContinue where you left off — workflow procedures are in your `lessmess-*` skills.")
	b.WriteString("\n\n-----\n\n")
	b.WriteString(text)
	s.compacts.consume(sessionID)
	return b.String()
}

