package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"lessmess/internal/model"
)

// The skills catalog (skills.go) only reaches agents when the served
// repository's opencode.json lists its URL under `skills`. This file
// patches that entry — order-preserving and idempotent, the same
// discipline as the Align button's default_agent patch (opendefault.go):
// unknown keys and their bytes survive untouched, and JSONC is refused
// rather than rewritten.

// EnsureSkillsCatalog points the served repository's opencode.json at
// this server's skill catalog. Best-effort by design: a failure logs
// and returns — the catalog itself keeps serving either way. Called
// from the boot closure once PublicBase is known (hub slots patch their
// own project file with the prefixed URL).
func (s *Server) EnsureSkillsCatalog() {
	url := s.apiBase() + "/skills/"
	path := opencodeConfigPath(s.st.Dir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		seed := fmt.Sprintf("{\"skills\":[%q]}", url)
		if werr := model.WriteFileAtomic(path, []byte(seed), 0o644); werr != nil {
			slog.Warn("skills catalog: cannot create opencode.json", "err", werr)
			return
		}
		slog.Info("opencode.json created with skills catalog", "url", url)
		return
	}
	if err != nil {
		slog.Warn("skills catalog: cannot read opencode.json", "err", err)
		return
	}
	patched, changed, perr := patchSkillsCatalog(data, url)
	if perr != nil {
		slog.Warn("skills catalog: opencode.json not patchable", "err", perr)
		return
	}
	if !changed {
		return
	}
	if werr := model.WriteFileAtomic(path, patched, 0o644); werr != nil {
		slog.Warn("skills catalog: cannot write opencode.json", "err", werr)
		return
	}
	slog.Info("opencode.json skills catalog entry written", "url", url)
}

// patchSkillsCatalog ensures data's top-level `skills` array contains
// url. It returns the (possibly identical) bytes and whether anything
// changed. Key order, whitespace, and unknown keys are preserved; JSONC
// input is an error because it could not be rewritten safely.
func patchSkillsCatalog(data []byte, url string) (patched []byte, changed bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, false, fmt.Errorf("not plain JSON: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, false, errors.New("not a JSON object")
	}
	type span struct{ start, end int }
	var skillsSpan *span
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, false, fmt.Errorf("not plain JSON: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, false, errors.New("unexpected key token")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, false, fmt.Errorf("not plain JSON: %w", err)
		}
		if key == "skills" {
			end := dec.InputOffset()
			s := span{start: int(end) - len(raw), end: int(end)}
			skillsSpan = &s
		}
	}
	if skillsSpan != nil {
		var entries []string
		if err := json.Unmarshal(data[skillsSpan.start:skillsSpan.end], &entries); err != nil {
			return nil, false, errors.New("skills is not an array of strings")
		}
		if slices.Contains(entries, url) {
			return data, false, nil // already correct: byte-identical no-op
		}
		// Entries ending in /skills/ are lessmess-managed catalog URLs —
		// replace them wholesale so a moved or re-ported server heals
		// instead of accumulating stale entries. Foreign entries survive.
		kept := entries[:0:0]
		for _, e := range entries {
			if !strings.HasSuffix(e, "/skills/") {
				kept = append(kept, e)
			}
		}
		kept = append(kept, url)
		lit, err := json.Marshal(kept)
		if err != nil {
			return nil, false, err
		}
		out := make([]byte, 0, len(data)-skillsSpan.end+skillsSpan.start+len(lit))
		out = append(out, data[:skillsSpan.start]...)
		out = append(out, lit...)
		out = append(out, data[skillsSpan.end:]...)
		return out, true, nil
	}
	// No skills key: insert one just before the object's closing brace,
	// directly after the last value and preserving trailing whitespace.
	close := bytes.LastIndexByte(data, '}')
	if close < 0 {
		return nil, false, errors.New("not a JSON object")
	}
	inner := data[:close]
	trim := bytes.TrimRight(inner, " \t\r\n")
	lit, err := json.Marshal([]string{url})
	if err != nil {
		return nil, false, err
	}
	var out []byte
	if bytes.Equal(bytes.TrimLeft(trim, " \t\r\n"), []byte("{")) {
		// Empty object: no leading comma.
		out = append(out, trim...)
		out = append(out, []byte("\n  \"skills\": ")...)
		out = append(out, lit...)
		out = append(out, '\n')
	} else {
		out = append(out, trim...)
		out = append(out, []byte(",\n  \"skills\": ")...)
		out = append(out, lit...)
		out = append(out, '\n')
	}
	out = append(out, inner[len(trim):]...)
	out = append(out, data[close:]...)
	return out, true, nil
}
