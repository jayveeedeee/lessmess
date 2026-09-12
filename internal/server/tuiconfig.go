package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// Embedded terminals run the real opencode TUI with a lessmess-managed CLI
// config so the board gets a chrome-free session (no tab strip, no sidebar)
// without touching the user's own cli.json. Field names follow the published
// schema https://opencode.ai/v2/cli.json.

const tuiConfigSchemaURL = "https://opencode.ai/v2/cli.json"

// tuiConfigOverrides are forced on top of the user's CLI config for embedded
// sessions: no persistent tab strip, sidebar always hidden.
var tuiConfigOverrides = map[string]map[string]any{
	"tabs":    {"enabled": false},
	"session": {"sidebar": "hide"},
}

// ensureTUIConfig merges the user's opencode CLI config with the embedded
// overrides and writes the result to <repoDir>/.lessmess/xdg/opencode/cli.json,
// returning the directory to export as XDG_CONFIG_HOME for the spawned TUI.
// The user's file is only ever read; a user file that cannot be merged
// degrades to an overrides-only config with a warning rather than an error.
func ensureTUIConfig(repoDir string) (string, error) {
	userPath := userCLIConfigPath()
	data, degraded, err := mergedTUIConfig(userPath)
	if err != nil {
		return "", err
	}
	if degraded {
		slog.Warn("tui config: cli.json not merged, using overrides only", "path", userPath)
	}
	xdg := filepath.Join(repoDir, store.StateDirName, "xdg")
	dir := filepath.Join(xdg, "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir tui config: %w", err)
	}
	if err := model.WriteFileAtomic(filepath.Join(dir, "cli.json"), data, 0o644); err != nil {
		return "", fmt.Errorf("write tui config: %w", err)
	}
	return xdg, nil
}

// xdgEnv returns the current process environment with XDG_CONFIG_HOME set to
// xdg, replacing any inherited entry so the override wins regardless of
// first- or last-match getenv semantics in the child.
func xdgEnv(xdg string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "XDG_CONFIG_HOME=") {
			env = append(env, e)
		}
	}
	return append(env, "XDG_CONFIG_HOME="+xdg)
}

// userCLIConfigPath resolves the user's cli.json the way opencode does:
// $XDG_CONFIG_HOME/opencode/cli.json, else ~/.config/opencode/cli.json.
func userCLIConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode", "cli.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("opencode", "cli.json") // misses; degrades to overrides-only
	}
	return filepath.Join(home, ".config", "opencode", "cli.json")
}

// mergedTUIConfig reads the user config at userPath (missing is fine) and
// returns it as indented JSON with the embedded overrides applied. degraded
// reports that an existing user file could not be merged and was dropped.
func mergedTUIConfig(userPath string) (data []byte, degraded bool, err error) {
	cfg := map[string]any{}
	switch b, readErr := os.ReadFile(userPath); {
	case errors.Is(readErr, os.ErrNotExist):
		// fresh overrides-only config
	case readErr != nil:
		degraded = true
	default:
		parsed, ok := parseCLIConfig(b)
		if !ok {
			parsed, ok = parseCLIConfig(stripJSONC(b))
		}
		if !ok {
			degraded = true
		} else {
			cfg = parsed
		}
	}
	for parent, kv := range tuiConfigOverrides {
		sub, ok := cfg[parent].(map[string]any)
		if !ok {
			sub = map[string]any{}
			cfg[parent] = sub
		}
		for k, v := range kv {
			sub[k] = v
		}
	}
	if _, ok := cfg["$schema"]; !ok {
		cfg["$schema"] = tuiConfigSchemaURL
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("marshal tui config: %w", err)
	}
	return append(out, '\n'), degraded, nil
}

// parseCLIConfig unmarshals a strict-JSON cli.json; a top-level null counts
// as an empty config, any other non-object as unparseable.
func parseCLIConfig(b []byte) (map[string]any, bool) {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, true
}

// stripJSONC removes // and /* */ comments and trailing commas outside string
// literals — a conservative best-effort pass so a hand-edited JSONC cli.json
// can still be merged.
func stripJSONC(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr, esc := false, false
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case inStr:
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			i++
		case c == '"':
			inStr = true
			out = append(out, c)
			i++
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i += 2 // an unterminated comment just runs out of input
		case c == ',':
			j := i + 1
			for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
				j++
			}
			if j < len(b) && (b[j] == '}' || b[j] == ']') {
				i++ // drop the trailing comma
			} else {
				out = append(out, c)
				i++
			}
		default:
			out = append(out, c)
			i++
		}
	}
	return out
}
