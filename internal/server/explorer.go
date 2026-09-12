package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"lessmess/internal/docs"
)

// --- explorer view model ---

type explorerEntry struct {
	Name  string
	Blurb string
}

type explorerNode struct {
	Rel     string
	Name    string
	Purpose string
	Dirs    []*explorerNode
	Files   []explorerEntry
}

type explorerView struct {
	Enabled  bool
	RepoName string
	Root     *explorerNode
}

// buildExplorerView walks the covered tree and attaches each directory's
// docs content. Disabled (no config) yields a zero view; walk errors degrade
// to an empty tree with a log line, never a failed page.
func (s *Server) buildExplorerView() explorerView {
	var v explorerView
	if s.docsQ == nil {
		return v
	}
	v.Enabled = true
	tree, err := docs.Walk(s.st.Dir, s.docsQ.cfg)
	if err != nil {
		slog.Warn("explorer walk", "err", err)
		return v
	}
	v.RepoName = filepath.Base(s.st.Dir)
	v.Root = buildExplorerNode(tree)
	return v
}

func buildExplorerNode(d *docs.Dir) *explorerNode {
	n := &explorerNode{Rel: d.Rel, Name: filepath.Base(d.Abs)}
	content, err := docs.DirDocs(d)
	if err != nil {
		slog.Warn("explorer docs", "dir", d.Rel, "err", err)
		content = &docs.DocsContent{}
	}
	n.Purpose = content.Purpose
	for _, c := range d.Children {
		n.Dirs = append(n.Dirs, buildExplorerNode(c))
	}
	for _, f := range d.Files {
		n.Files = append(n.Files, explorerEntry{Name: f, Blurb: content.Blurbs[f]})
	}
	return n
}

// --- handlers ---

// explorer renders GET /explorer: the full page.
func (s *Server) explorer(w http.ResponseWriter, r *http.Request) {
	s.rend.render(w, s.rend.explorer, "layout", pageData{Title: "explorer", Page: "explorer", Data: s.buildExplorerView()})
}

// explorerTree renders GET /explorer/tree: just the tree fragment, for
// htmx swaps on docs SSE events.
func (s *Server) explorerTree(w http.ResponseWriter, r *http.Request) {
	s.rend.render(w, s.rend.partial, "explorerTree", s.buildExplorerView())
}

// findExplorerNode returns the node with the given relative path, or nil.
func findExplorerNode(n *explorerNode, rel string) *explorerNode {
	if n == nil {
		return nil
	}
	if n.Rel == rel {
		return n
	}
	for _, c := range n.Dirs {
		if found := findExplorerNode(c, rel); found != nil {
			return found
		}
	}
	return nil
}

// explorerDetail renders GET /explorer/detail?dir=<rel>: the detail-pane
// fragment for one covered directory, swapped in by htmx on selection.
func (s *Server) explorerDetail(w http.ResponseWriter, r *http.Request) {
	if s.docsQ == nil {
		http.Error(w, "docs system disabled", http.StatusServiceUnavailable)
		return
	}
	dir := path.Clean(filepath.ToSlash(strings.TrimSpace(r.URL.Query().Get("dir"))))
	if dir == "" {
		dir = "."
	}
	if dir != "." && !s.docsQ.cfg.Covered(dir) {
		http.Error(w, "not a covered directory: "+dir, http.StatusUnprocessableEntity)
		return
	}
	if fi, err := os.Stat(filepath.Join(s.st.Dir, filepath.FromSlash(dir))); err != nil || !fi.IsDir() {
		http.Error(w, "directory does not exist: "+dir, http.StatusUnprocessableEntity)
		return
	}
	tree, err := docs.Walk(s.st.Dir, s.docsQ.cfg)
	if err != nil {
		slog.Warn("explorer detail walk", "err", err)
		http.Error(w, "walk failed", http.StatusInternalServerError)
		return
	}
	node := findExplorerNode(buildExplorerNode(tree), dir)
	if node == nil {
		slog.Warn("explorer detail lookup", "dir", dir)
		http.Error(w, "directory not in covered tree: "+dir, http.StatusNotFound)
		return
	}
	s.rend.render(w, s.rend.partial, "explorerDetail", node)
}

// explorerChat handles POST /explorer/chat: create a root-scoped opencode
// session instructed to answer questions about one covered directory, map it
// to the unassigned bucket (Discussions list), and return the session ID.
func (s *Server) explorerChat(w http.ResponseWriter, r *http.Request) {
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.docsQ == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "docs system disabled"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	var req struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	dir := path.Clean(filepath.ToSlash(strings.TrimSpace(req.Dir)))
	if dir == "" {
		dir = "."
	}
	if dir != "." && !s.docsQ.cfg.Covered(dir) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "not a covered directory: " + dir})
		return
	}
	if fi, err := os.Stat(filepath.Join(s.st.Dir, filepath.FromSlash(dir))); err != nil || !fi.IsDir() {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "directory does not exist: " + dir})
		return
	}

	structure := readDocForPrompt(filepath.Join(s.st.Dir, filepath.FromSlash(dir), docs.StructureFile))
	agents := readDocForPrompt(filepath.Join(s.st.Dir, filepath.FromSlash(dir), docs.AgentsFile))
	title := "explore " + dir

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	sess, err := s.oc.CreateSession(ctx, title, s.st.Dir)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	if err := s.oc.Prompt(ctx, sess.ID, explorerPrompt(dir, structure, agents)); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "prime explorer session: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.addUnassigned(entry); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("explorer session created", "dir", dir, "session", sess.ID)
	writeJSON(w, http.StatusCreated, sessionResponse{Session: entry.Session, Title: entry.Title, Created: entry.Created, Live: true})
}

// readDocForPrompt returns a doc file's content for prompt embedding.
func readDocForPrompt(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "(not present)"
	}
	return strings.TrimSpace(string(data))
}

// explorerPrompt builds the prompt priming an explorer chat: root working
// context, directory focus, both doc files embedded for grounding.
func explorerPrompt(dir, structure, agents string) string {
	return fmt.Sprintf(`You are answering questions about one directory of this repository.

You are working in the repository ROOT, so you can read anything in the repo, but your focus is:

Directory: %[1]s

Its STRUCTURE.md:
---
%[2]s
---

Its AGENTS.md:
---
%[3]s
---

Answer the user's questions about %[1]s: its purpose, its files, and how it fits into the repository. Read code as needed and stay grounded in what you actually read. Do not modify any files unless the user explicitly asks you to.`, dir, structure, agents)
}
