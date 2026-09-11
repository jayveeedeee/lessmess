package model

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

// TaskFile is a parsed changes/<id>/tasks/NN-*.md file.
type TaskFile struct {
	ID    string
	Title string
	Body  string // markdown content after the frontmatter
}

// ParseTaskFile parses a task file's frontmatter. Per AGENTS.md the
// frontmatter must contain exactly id and title.
func ParseTaskFile(name string, data []byte) (*TaskFile, error) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return nil, parseErr(name, "task file must start with YAML frontmatter (---)")
	}
	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	tail := len("\n---\n")
	if end < 0 {
		if strings.HasSuffix(rest, "\n---") {
			end = len(rest) - len("\n---")
			tail = len("\n---")
		} else {
			return nil, parseErr(name, "frontmatter closing --- not found")
		}
	}
	fm := rest[:end]
	body := rest[end+tail:]
	var meta struct {
		ID    string `yaml:"id"`
		Title string `yaml:"title"`
	}
	dec := yaml.NewDecoder(bytes.NewReader([]byte(fm)))
	dec.KnownFields(true)
	if err := dec.Decode(&meta); err != nil {
		return nil, parseErr(name, "frontmatter: %v (only id and title are allowed)", err)
	}
	if meta.ID == "" || meta.Title == "" {
		return nil, parseErr(name, "frontmatter must contain id and title")
	}
	return &TaskFile{ID: meta.ID, Title: meta.Title, Body: body}, nil
}
