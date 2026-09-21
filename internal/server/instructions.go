package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// The instruction-injection engine: session primes are composed from
// versioned modules embedded in the binary, selected deterministically by
// session kind and change/task state, with the current state snapshot
// injected so agents never parse state files. Refining an instruction
// means updating this file and rebuilding — no repository file changes.

//go:embed instructions.json
var instructionsJSON []byte

// InstructionModule is one versioned instruction payload. Audiences name
// the session kinds that receive it ("discussion", "change", "task");
// When optionally gates it further ("worktree" = worktree-backed changes
// only). Text carries {{placeholders}} the engine substitutes.
type InstructionModule struct {
	ID        string   `json:"id"`
	Version   int      `json:"version"`
	Audiences []string `json:"audiences"`
	When      string   `json:"when,omitempty"`
	Text      string   `json:"text"`
}

// instructionSet is the parsed instructions.json.
type instructionSet struct {
	Version int                 `json:"version"`
	Modules []InstructionModule `json:"modules"`
}

var instructions = mustLoadInstructions()

func mustLoadInstructions() []InstructionModule {
	var set instructionSet
	if err := json.Unmarshal(instructionsJSON, &set); err != nil {
		panic("instructions: " + err.Error())
	}
	if set.Version != 1 {
		panic(fmt.Sprintf("instructions: unsupported manifest version %d", set.Version))
	}
	ids := map[string]bool{}
	for _, m := range set.Modules {
		if m.ID == "" || m.Text == "" || len(m.Audiences) == 0 {
			panic("instructions: module missing id, text, or audiences: " + m.ID)
		}
		if ids[m.ID] {
			panic("instructions: duplicate module id " + m.ID)
		}
		ids[m.ID] = true
	}
	return set.Modules
}

// primeContext carries the deterministic substitution values and state
// flags for one prime render.
type primeContext struct {
	APIBase        string
	ChangeID       string
	SessionID      string
	TaskID         string
	TaskHref       string
	Worktree       string // non-empty → the worktree module applies
	WorktreeBranch string
	Snapshot       string // rendered state view appended to the prime
}

// selectModules returns the manifest-ordered modules for one audience and
// context. Selection is a pure function of (manifest, audience, flags).
func selectModules(audience string, pc primeContext) []InstructionModule {
	var out []InstructionModule
	for _, m := range instructions {
		if !containsAudience(m.Audiences, audience) {
			continue
		}
		switch m.When {
		case "", "always":
		case "worktree":
			if pc.Worktree == "" {
				continue
			}
		default:
			// Unknown conditions never fire; the manifest is validated
			// by tests so this cannot happen silently.
			continue
		}
		out = append(out, m)
	}
	return out
}

func containsAudience(audiences []string, want string) bool {
	for _, a := range audiences {
		if a == want {
			return true
		}
	}
	return false
}

// renderModule substitutes the engine's placeholders.
func renderModule(m InstructionModule, pc primeContext) string {
	r := strings.NewReplacer(
		"{{apiBase}}", pc.APIBase,
		"{{changeId}}", pc.ChangeID,
		"{{sessionId}}", pc.SessionID,
		"{{taskId}}", pc.TaskID,
		"{{taskHref}}", pc.TaskHref,
		"{{worktree}}", pc.Worktree,
		"{{worktreeBranch}}", pc.WorktreeBranch,
	)
	return r.Replace(m.Text)
}

// renderPrime composes the full prime for one audience: the selected
// modules in manifest order, then the injected state snapshot. It returns
// the text and the selected module IDs for audit logging.
func renderPrime(audience string, pc primeContext) (string, []string) {
	var b strings.Builder
	var ids []string
	for _, m := range selectModules(audience, pc) {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(renderModule(m, pc))
		ids = append(ids, m.ID)
	}
	if pc.Snapshot != "" {
		b.WriteString("\n\n----- Current state (tool-injected; authoritative) -----\n\n")
		b.WriteString(pc.Snapshot)
	}
	return b.String(), ids
}

// instructionManifest serves GET /workflow/instructions: the module list
// with versions and audiences for debugging and audit.
func instructionManifest() map[string]any {
	type entry struct {
		ID        string   `json:"id"`
		Version   int      `json:"version"`
		Audiences []string `json:"audiences"`
		When      string   `json:"when,omitempty"`
	}
	entries := make([]entry, 0, len(instructions))
	for _, m := range instructions {
		entries = append(entries, entry{ID: m.ID, Version: m.Version, Audiences: m.Audiences, When: m.When})
	}
	audiences := map[string][]string{}
	for _, m := range instructions {
		for _, a := range m.Audiences {
			audiences[a] = append(audiences[a], m.ID)
		}
	}
	for a := range audiences {
		sort.Strings(audiences[a])
	}
	return map[string]any{"version": 1, "modules": entries, "selection": audiences}
}

// logPrime records the injected module set at spawn for auditability.
func logPrime(sessionID, audience string, ids []string) {
	slog.Info("session primed", "session", sessionID, "audience", audience, "modules", strings.Join(ids, ","))
}
