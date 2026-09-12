// Package docs implements repo docs management: coverage configuration,
// STRUCTURE.md generation, seeding, and refresh orchestration for the
// per-folder AGENTS.md/STRUCTURE.md doc pairs.
package docs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ConfigFile is the committed coverage-configuration file at the repo root.
// When it is absent the docs system is disabled and all consumers no-op.
const ConfigFile = "agentsdocs.json"

// DefaultExclude always applies; user excludes are added on top. These are
// generated/dependency directories where docs would be noise.
var DefaultExclude = []string{"node_modules", "vendor", "dist", "build", "out", "target"}

// Config decides which directories get the AGENTS.md/STRUCTURE.md doc pair.
//
// Patterns are slash-separated globs evaluated against the repo-relative
// directory path ("internal/model"). A pattern without a slash matches a
// directory's base name at any depth; a pattern with a slash matches the
// full relative path. A "**" segment matches any number of path segments.
type Config struct {
	Include []string `json:"include"` // default ["**"]: every dir is a candidate
	Exclude []string `json:"exclude"` // added to DefaultExclude; exclude wins

	inc []pattern
	exc []pattern
}

// DefaultConfig returns the config written by `tasktracker init`: cover
// everything subject to the built-in exclusions.
func DefaultConfig() *Config {
	c := &Config{Include: []string{"**"}, Exclude: []string{}}
	_ = c.compile() // built-in patterns are valid by construction
	return c
}

// LoadConfig reads root/agentsdocs.json. It returns nil without error when
// the file does not exist: the docs system is disabled for that repo.
func LoadConfig(root string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(root, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", ConfigFile, err)
	}
	if len(c.Include) == 0 {
		c.Include = []string{"**"}
	}
	if err := c.compile(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) compile() error {
	var err error
	c.inc, err = compileAll(c.Include)
	if err != nil {
		return err
	}
	c.exc, err = compileAll(append(append([]string{}, DefaultExclude...), c.Exclude...))
	return err
}

func compileAll(raws []string) ([]pattern, error) {
	var ps []pattern
	for _, raw := range raws {
		p, err := compilePattern(raw)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, nil
}

// Covered reports whether the directory at repo-relative path rel ("/"- or
// OS-separated, "." for the root) gets the doc pair.
//
// The root is always covered when a config exists. Hidden directories (any
// path segment starting with ".", which includes .git and .tasktracker) and
// the changes/ tree are never covered: changes/ holds only canonical
// workflow data per AGENTS.md.
func (c *Config) Covered(rel string) bool {
	rel = filepath.ToSlash(rel)
	if rel == "" || rel == "." {
		return true
	}
	rel = strings.Trim(rel, "/")
	segs := strings.Split(rel, "/")
	for _, s := range segs {
		if strings.HasPrefix(s, ".") {
			return false
		}
	}
	if segs[0] == "changes" {
		return false
	}
	return matchAny(c.inc, segs) && !matchAny(c.exc, segs)
}

type pattern struct {
	baseOnly bool // no slash: match the path's base name at any depth
	segs     []string
}

func compilePattern(raw string) (pattern, error) {
	raw = strings.Trim(filepath.ToSlash(raw), "/")
	if raw == "" {
		return pattern{}, fmt.Errorf("%s: empty pattern", ConfigFile)
	}
	p := pattern{baseOnly: !strings.Contains(raw, "/"), segs: strings.Split(raw, "/")}
	for _, s := range p.segs {
		if s == "**" {
			continue
		}
		if _, err := path.Match(s, ""); err != nil {
			return pattern{}, fmt.Errorf("%s: bad pattern %q: %w", ConfigFile, raw, err)
		}
	}
	return p, nil
}

func matchAny(ps []pattern, segs []string) bool {
	for _, p := range ps {
		if p.matches(segs) {
			return true
		}
	}
	return false
}

func (p pattern) matches(segs []string) bool {
	if p.baseOnly {
		ok, _ := path.Match(p.segs[0], segs[len(segs)-1])
		return ok
	}
	return matchSegments(p.segs, segs)
}

// matchSegments matches pattern segments against path segments; "**" matches
// zero or more segments.
func matchSegments(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegments(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pat[1:], segs[1:])
}
