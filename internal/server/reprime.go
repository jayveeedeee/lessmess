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
// by construction — but the personalized parts do not, so a bound session
// whose compaction has not been compensated gets a re-prime: the binding
// line, a fresh state snapshot, and a pointer back to the skills.
//
// The vehicle is a service synthetic message (POST
// /api/session/{id}/synthetic, resume false), the same mechanism the
// service uses for its own instruction updates — never a rewrite of the
// user's prompt. The synthetic is admitted to the session inbox and folds
// into the next execution ahead of the input that triggers it, so no model
// turn is spent on the restoration and the user's message — sent or
// queued — always goes through verbatim.
//
// Detection is stateless: a bound session needs a re-prime iff its newest
// completed compaction message is strictly newer than its newest user
// message or delivered restoration synthetic, and no restoration synthetic
// is already pending in the inbox. The pending check closes the queue
// blindness of the prompt-rewrite design (queued prompts never land in the
// transcript, so transcript-only compensation loops); the injected
// synthetic becomes visible both ways — inbox while pending, transcript
// once delivered — making the rule self-terminating without any memory.

// reprimeMarker prefixes the restoration text; the rule recognizes
// messages carrying it as compensation. It must stay stable: transcripts
// hold it durably.
const reprimeMarker = "Context restoration:"

// reprimeScanLimit bounds the descending transcript read behind the
// re-prime rule. The newest user message and the newest completed
// compaction sit at the transcript tail, separated only by the final
// turn's assistant and tool parts, so a single page is enough and the
// rule never paginates.
const reprimeScanLimit = 50

// needsReprime evaluates the rule: walk the newest page newest-first for
// the newest user message, restoration synthetic, or completed compaction.
// A compaction strictly newer than the newest of those means the context
// was compacted with nothing compensating since; equal timestamps count
// as already continued (a missed re-prime is safer than a spurious one).
// A page exhausted on a completed compaction with no compensator is a
// conservative re-prime; exhaustion without one means nothing needs
// compensating. A marker synthetic already pending in the inbox
// compensates too. Any read failure fails open.
func (s *Server) needsReprime(ctx context.Context, sessionID string, cap opencode.LifecycleCapabilities) bool {
	if s.oc == nil {
		return false
	}
	if cap.InboxList {
		items, err := s.oc.ListInbox(ctx, cap, sessionID)
		if err == nil {
			for _, item := range items {
				if item.Type == "synthetic" && strings.HasPrefix(item.Text, reprimeMarker) {
					return false
				}
			}
		} else {
			slog.Warn("reprime rule: inbox unreadable", "session", sessionID, "err", err)
		}
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
		case m.Type == "user" || (m.Type == "synthetic" && strings.HasPrefix(m.Text, reprimeMarker)):
			return compacted && m.Time.Created < compactedAt
		case m.Type == "compaction" && m.Status == "completed":
			compacted = true
			compactedAt = m.Time.Created
		}
	}
	return compacted
}

// ensureReprime compensates an uncompensated compaction on a bound session
// by injecting the restoration synthetic. Unbound sessions are never
// touched (a free chat has no binding to restore), and any lookup, read,
// capability, or injection failure fails open — the user's prompt is never
// blocked or modified by this path.
func (s *Server) ensureReprime(ctx context.Context, sessionID string) {
	if s.oc == nil || s.mapErr != nil {
		return
	}
	changeID, entry, bound := s.sessions.entry(sessionID)
	if !bound {
		return
	}
	cap, err := s.oc.LifecycleCapabilities(ctx)
	if err != nil {
		slog.Warn("reprime skipped: capabilities unreadable", "session", sessionID, "err", err)
		return
	}
	if !s.needsReprime(ctx, sessionID, cap) {
		return
	}
	var b strings.Builder
	b.WriteString(reprimeMarker + " the conversation above was compacted. ")
	if entry.Task != "" {
		b.WriteString("This session is bound to task " + entry.Task + " of change " + changeID + ".")
	} else {
		b.WriteString("This session is bound to change " + changeID + ".")
	}
	snapshot, err := s.st.LedgerFile(changeID)
	if err != nil {
		slog.Warn("reprime skipped: ledger unreadable", "change", changeID, "err", err)
		return
	}
	b.WriteString("\n\nCurrent state (tool-injected; authoritative):\n\n")
	b.WriteString(snapshot)
	b.WriteString("\n\nContinue where you left off — workflow procedures are in your `lessmess-*` skills.")
	if _, err := s.oc.AddSynthetic(ctx, cap, sessionID, b.String()); err != nil {
		slog.Warn("reprime skipped: synthetic injection failed", "session", sessionID, "err", err)
	}
}
