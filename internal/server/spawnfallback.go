package server

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// spawnFallbackFile holds the last spawn-fallback record under .lessmess/,
// one state file per feature per the tooling-state convention.
const spawnFallbackFile = "spawn-fallback.json"

// SpawnFallback records a session spawn that succeeded only after
// de-escalating the configured agent/model. It is the user-visible trace
// behind the board banner: without it a fallback would explain itself only
// in the server log while sessions visibly run under a different agent.
type SpawnFallback struct {
	Time           string `json:"time"`
	AttemptedAgent string `json:"attemptedAgent,omitempty"`
	AttemptedModel string `json:"attemptedModel,omitempty"`
	// Outcome names the ladder step that succeeded: agent-only,
	// model-only, or plain.
	Outcome      string `json:"outcome"`
	ServiceError string `json:"serviceError"`
}

func spawnFallbackPath(repoDir string) string {
	return filepath.Join(repoDir, store.StateDirName, spawnFallbackFile)
}

// readSpawnFallback loads the record; nil when absent or malformed — a
// banner must never break the index, so reads fail open.
func readSpawnFallback(repoDir string) *SpawnFallback {
	data, err := os.ReadFile(spawnFallbackPath(repoDir))
	if err != nil {
		return nil
	}
	var rec SpawnFallback
	if err := json.Unmarshal(data, &rec); err != nil || rec.Outcome == "" {
		return nil
	}
	return &rec
}

// writeSpawnFallback persists rec, creating .lessmess/ as needed. Best
// effort: a failed write costs the banner, never the session.
func writeSpawnFallback(repoDir string, rec SpawnFallback) {
	if rec.Time == "" {
		rec.Time = time.Now().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		slog.Warn("spawn-fallback record marshal failed", "err", err)
		return
	}
	data = append(data, '\n')
	dir := filepath.Join(repoDir, store.StateDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("spawn-fallback record write failed", "err", err)
		return
	}
	if err := model.WriteFileAtomic(filepath.Join(dir, spawnFallbackFile), data, 0o644); err != nil {
		slog.Warn("spawn-fallback record write failed", "err", err)
	}
}

// clearSpawnFallback removes the record; called on the next clean spawn so
// the banner self-heals without a dismissal state.
func clearSpawnFallback(repoDir string) {
	if err := os.Remove(spawnFallbackPath(repoDir)); err != nil && !os.IsNotExist(err) {
		slog.Warn("spawn-fallback record clear failed", "err", err)
	}
}

// Summary renders the banner sentence for the record's outcome.
func (f *SpawnFallback) Summary() string {
	switch f.Outcome {
	case "agent-only":
		return "session created with agent “" + f.AttemptedAgent + "” only — model “" +
			f.AttemptedModel + "” was rejected (" + f.ServiceError + ")"
	case "model-only":
		return "session created with model “" + f.AttemptedModel + "” only — agent “" +
			f.AttemptedAgent + "” was rejected (" + f.ServiceError + ")"
	default:
		return "session created without the configured agent/model (" + f.ServiceError + ")"
	}
}
