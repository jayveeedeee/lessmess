package server

import (
	"context"
	"log/slog"
	"strings"

	"lessmess/internal/opencode"
)

// Compaction re-prime: when a session's context is compacted, the base
// prompt (binding facts) and the state snapshot are summarized away.
// Capabilities need no restore — the skill listing survives compaction
// by construction — but the personalized parts do not, so the next
// prompt to a bound session carries a compact re-prime: the binding
// line, a fresh state snapshot, and a pointer back to the skills.
//
// Detection is stateless and transcript-based: a bound session needs a
// re-prime iff its newest completed compaction message is strictly
// newer than its newest user message. One small descending page read at
// prompt time decides, so the rule is restart-proof (no memory, nothing
// persisted), ignores failed compactions (`failed` never counts),
// self-heals across fork/revert, and cannot double-fire on an in-flight
// compaction: a queued compaction counts only once completed, and the
// first prompt after that completion is the single wrap.

// reprimeScanLimit bounds the descending transcript read behind the
// re-prime rule. The newest user message and the newest completed
// compaction sit at the transcript tail, separated only by the final
// turn's assistant and tool parts, so a single page is enough and the
// rule never paginates.
const reprimeScanLimit = 50

// needsReprime evaluates the transcript rule: walk the newest page
// newest-first. The first user message or completed compaction
// encountered is the newest of its kind, so the walk records the newest
// completed compaction and decides at the first user message — a
// compaction strictly newer than it means the context was compacted
// with no user turn since; equal timestamps count as already continued
// (a missed re-prime is safer than a spurious one). A page exhausted on
// a completed compaction with no user message is a conservative
// re-prime; exhaustion without one means nothing needs compensating.
// Any read failure fails open.
func (s *Server) needsReprime(ctx context.Context, sessionID string) bool {
	if s.oc == nil {
		return false
	}
	page, err := s.oc.ListMessagesPage(ctx, sessionID, opencode.ListMessagesOptions{Limit: reprimeScanLimit, Order: "desc"})
	if err != nil {
		slog.Warn("reprime skipped: transcript unreadable", "session", sessionID, "err", err)
		return false
	}
	var compactedAt float64
	compacted := false
	for i := range page.Messages {
		m := &page.Messages[i]
		switch {
		case m.Type == "user":
			return compacted && m.Time.Created < compactedAt
		case m.Type == "compaction" && m.Status == "completed":
			compacted = true
			compactedAt = m.Time.Created
		}
	}
	return compacted
}

// maybeReprime returns the prompt text to send: either the caller's text
// unchanged, or the re-prime block followed by it. Unbound sessions are
// never wrapped (a free chat has no binding to restore), and any
// lookup or read failure fails open — a user prompt is never blocked by
// this path.
func (s *Server) maybeReprime(ctx context.Context, sessionID, text string) string {
	if s.mapErr != nil {
		return text
	}
	changeID, entry, bound := s.sessions.entry(sessionID)
	if !bound || !s.needsReprime(ctx, sessionID) {
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
	return b.String()
}
