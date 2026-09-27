package server

import (
	_ "embed"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// The workflow-skills HTTP catalog: OpenCode V2 loads skills from a URL
// (the `skills` array in opencode.json) whose base holds an index.json
// plus one markdown file per skill. Serving the bodies from here keeps a
// single source of truth (this file's embedded manifest) and makes the
// capability tier available to every session — the listing survives
// compaction, so agents can always reload procedures the conversation
// lost. The content-hash version makes OpenCode refresh its cache when
// the bodies change.

//go:embed skills.json
var skillsJSON []byte

// skillDef is one catalog entry: the skill ID is the Name (the entry
// file is <name>.md, which OpenCode maps to that ID).
type skillDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

type skillSet struct {
	Version int        `json:"version"`
	Skills  []skillDef `json:"skills"`
}

var skillDefs = mustLoadSkills()

func mustLoadSkills() []skillDef {
	var set skillSet
	if err := json.Unmarshal(skillsJSON, &set); err != nil {
		panic("skills: " + err.Error())
	}
	if len(set.Skills) == 0 {
		panic("skills: empty manifest")
	}
	seen := map[string]bool{}
	for _, s := range set.Skills {
		if s.Name == "" || s.Description == "" || s.Body == "" {
			panic("skills: entry missing name/description/body: " + s.Name)
		}
		if seen[s.Name] {
			panic("skills: duplicate name " + s.Name)
		}
		seen[s.Name] = true
	}
	return set.Skills
}

// skillCatalogVersion hashes the catalog's raw content; editing any
// body or description changes the version OpenCode sees. Raw (rather
// than rendered) content keeps the version independent of the serving
// base URL.
func skillCatalogVersion() string {
	h := sha256.New()
	for _, s := range skillDefs {
		h.Write([]byte(s.Name))
		h.Write([]byte{0})
		h.Write([]byte(s.Description))
		h.Write([]byte{0})
		h.Write([]byte(s.Body))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// renderSkillBody substitutes the catalog's placeholders. Bodies are
// deliberately session-agnostic (no change/task placeholders): the same
// bytes serve every session of a project.
func renderSkillBody(body, apiBase string) string {
	return strings.ReplaceAll(body, "{{apiBase}}", apiBase)
}

// skillIndex handles GET /skills/index.json: the catalog manifest.
func (s *Server) skillIndex(w http.ResponseWriter, r *http.Request) {
	version := skillCatalogVersion()
	type entry struct {
		Name    string   `json:"name"`
		Version string   `json:"version"`
		Files   []string `json:"files"`
	}
	entries := make([]entry, 0, len(skillDefs))
	for _, def := range skillDefs {
		entries = append(entries, entry{
			Name:    def.Name,
			Version: version,
			Files:   []string{def.Name + ".md"},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": entries})
}

// skillFile handles GET /skills/{name}/{file}: one skill's markdown,
// frontmatter plus body. Only the canonical <name>.md path is served.
func (s *Server) skillFile(w http.ResponseWriter, r *http.Request) {
	name, file := r.PathValue("name"), r.PathValue("file")
	var def *skillDef
	for i := range skillDefs {
		if skillDefs[i].Name == name {
			def = &skillDefs[i]
			break
		}
	}
	if def == nil || file != name+".md" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown skill"})
		return
	}
	apiBase := s.apiBase()
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + skillDisplayName(def.Name) + "\n")
	b.WriteString("description: " + def.Description + "\n")
	b.WriteString("---\n\n")
	b.WriteString(renderSkillBody(def.Body, apiBase))
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

// skillDisplayName turns the kebab-case ID into a display label:
// "lessmess-scaffold" → "Lessmess Scaffold".
func skillDisplayName(name string) string {
	words := strings.Fields(strings.ReplaceAll(name, "-", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
