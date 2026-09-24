// Package registry owns the user-level project registry: the list of
// repository directories one lessmess instance serves. The registry lives
// outside any repository (a global config under the user's config
// directory) because it describes the instance, not a project.
//
// The file is fail-open like all lessmess state: a missing or malformed
// file reads as an empty registry (malformed logs a warning; the next Save
// replaces it). Display names are deliberately not stored — they derive
// from each project's own settings at render time.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lessmess/internal/model"
)

// Project is one registered repository directory.
type Project struct {
	// Slug is the URL-safe identifier used in /p/<slug>/ mounts. Unique
	// within the registry.
	Slug string `json:"slug"`
	// Path is the absolute repository root.
	Path string `json:"path"`
	// Added is the registration date (YYYY-MM-DD).
	Added string `json:"added"`
}

// Config is the on-disk registry document.
type Config struct {
	Projects []Project `json:"projects"`
}

// Sentinel errors surfaced to callers that turn them into HTTP statuses.
var (
	// ErrInvalidPath: empty or relative path.
	ErrInvalidPath = errors.New("path must be absolute")
	// ErrMissing: path does not exist.
	ErrMissing = errors.New("directory does not exist")
	// ErrNotDir: path exists but is not a directory.
	ErrNotDir = errors.New("path is not a directory")
	// ErrDuplicate: the same directory is already registered.
	ErrDuplicate = errors.New("directory is already registered")
)

// EnvVar overrides the registry location (used by tests and unusual
// server setups).
const EnvVar = "LESSMESS_CONFIG"

// DefaultPath resolves the registry file location:
// $LESSMESS_CONFIG, or $XDG_CONFIG_HOME/lessmess/config.json, defaulting
// to ~/.config/lessmess/config.json on every platform (the same
// convention the opencode service uses for its own user-level config).
func DefaultPath() (string, error) {
	if p := os.Getenv(EnvVar); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "lessmess", "config.json"), nil
}

// Load reads the registry file, failing open: a missing file yields an
// empty config, a malformed file yields an empty config plus a warning
// (the next Save replaces it).
func Load(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("registry unreadable; treating as empty", "path", path, "err", err)
		}
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Warn("registry malformed; treating as empty", "path", path, "err", err)
		return Config{}
	}
	return cfg
}

// Save writes the registry atomically, creating parent directories.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return model.WriteFileAtomic(path, append(out, '\n'), 0o644)
}

// maxSlugLen caps generated slugs so long folder names stay readable URLs.
const maxSlugLen = 40

// Slugify converts a folder name into a URL-safe slug: lowercased, runs of
// non-alphanumeric characters collapsed to single dashes, trimmed, length-
// capped. A name with no usable characters becomes "project".
func Slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		default:
			dash = true
		}
		if b.Len() >= maxSlugLen {
			break
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "project"
	}
	return slug
}

// Add validates dir, allocates a unique slug, and appends the project.
// The path is cleaned; duplicates (same cleaned path) are rejected.
func Add(cfg *Config, dir string) (Project, error) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if !filepath.IsAbs(dir) {
		return Project{}, ErrInvalidPath
	}
	st, err := os.Stat(dir)
	if err != nil {
		return Project{}, ErrMissing
	}
	if !st.IsDir() {
		return Project{}, ErrNotDir
	}
	for _, p := range cfg.Projects {
		if p.Path == dir {
			return Project{}, ErrDuplicate
		}
	}
	slug := uniqueSlug(cfg, Slugify(filepath.Base(dir)))
	p := Project{Slug: slug, Path: dir, Added: time.Now().Format("2006-01-02")}
	cfg.Projects = append(cfg.Projects, p)
	return p, nil
}

// uniqueSlug returns slug, or slug-2, -3, … until free within cfg.
func uniqueSlug(cfg *Config, slug string) string {
	taken := make(map[string]bool, len(cfg.Projects))
	for _, p := range cfg.Projects {
		taken[p.Slug] = true
	}
	if !taken[slug] {
		return slug
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", slug, i)
		if !taken[candidate] {
			return candidate
		}
	}
}

// Get returns the project with the given slug.
func Get(cfg Config, slug string) (Project, bool) {
	for _, p := range cfg.Projects {
		if p.Slug == slug {
			return p, true
		}
	}
	return Project{}, false
}

// Remove deletes the project with the given slug; reports whether a
// project was removed.
func Remove(cfg *Config, slug string) bool {
	for i, p := range cfg.Projects {
		if p.Slug == slug {
			cfg.Projects = append(cfg.Projects[:i], cfg.Projects[i+1:]...)
			return true
		}
	}
	return false
}

// Store is a mutex-guarded registry file bound to one path — the hub's
// single writer handle. Every operation is load → mutate → save so the
// file on disk stays authoritative and survives external edits between
// calls.
type Store struct {
	mu   sync.Mutex
	path string
}

// OpenStore binds a registry Store to path.
func OpenStore(path string) *Store { return &Store{path: path} }

// Path is the backing file location.
func (s *Store) Path() string { return s.path }

// List returns the current registry contents.
func (s *Store) List() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Load(s.path)
}

// Get returns one project by slug.
func (s *Store) Get(slug string) (Project, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Get(Load(s.path), slug)
}

// Add validates and registers dir, persisting immediately.
func (s *Store) Add(dir string) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := Load(s.path)
	p, err := Add(&cfg, dir)
	if err != nil {
		return Project{}, err
	}
	if err := Save(s.path, cfg); err != nil {
		return Project{}, err
	}
	return p, nil
}

// Remove unregisters the project with the given slug, persisting
// immediately; reports whether a project was removed.
func (s *Store) Remove(slug string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := Load(s.path)
	if !Remove(&cfg, slug) {
		return false
	}
	return Save(s.path, cfg) == nil
}
