package server

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	"lessmess/internal/model"
	"lessmess/internal/store"
	"lessmess/web"
)

// --- templates ---

// md enables auto heading IDs (TOC anchors) and GFM tables (the ledger is
// table-shaped, so it needs table rendering to be readable in the modal).
var md = goldmark.New(
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithExtensions(extension.Table),
)

func renderMarkdown(s string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}
	return template.HTML(buf.String())
}

var templateFuncs = template.FuncMap{
	"statusClass": func(s string) string {
		return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
	},
	// statusRank maps a task status to its workflow-order index so the
	// index table can sort the Status column in board order (unknown
	// statuses sort last, deterministically).
	"statusRank": func(s string) int {
		for i, st := range model.TaskStatusOrder {
			if string(st) == s {
				return i
			}
		}
		return len(model.TaskStatusOrder)
	},
	"base":     filepath.Base,
	"markdown": renderMarkdown,
}

type renderer struct {
	index    *template.Template
	board    *template.Template
	partial  *template.Template
	explorer *template.Template
	settings *template.Template
	opencode *template.Template
	setup    *template.Template
	assetsV  string
	// accent resolves the palette entry for the page head; nil until the
	// owning server wires it (then defaults to the built-in orange).
	accent func() AccentColor
	// projectName resolves the project display name for the header and
	// tab title; nil until wired (then renders empty).
	projectName func() string
}

func mustParse(files ...string) *template.Template {
	return template.Must(template.New("page").Funcs(templateFuncs).ParseFS(web.FS, files...))
}

// assetsVersion returns a short content hash of the app-owned static
// assets, used for cache-busting (?v=) so browsers never run stale JS/CSS
// under the immutable cache policy. Vendored pinned libraries keep their
// own fixed versions.
func assetsVersion() string {
	h := fnv.New32a()
	for _, name := range []string{"static/app.css", "static/app.js"} {
		if b, err := fs.ReadFile(web.FS, name); err == nil {
			h.Write(b)
		}
	}
	return fmt.Sprintf("%x", h.Sum32())
}

func newRenderer() *renderer {
	return &renderer{
		index:    mustParse("templates/layout.html", "templates/index.html"),
		board:    mustParse("templates/layout.html", "templates/board.html", "templates/partials.html"),
		partial:  mustParse("templates/partials.html", "templates/explorer.html"),
		explorer: mustParse("templates/layout.html", "templates/explorer.html"),
		settings: mustParse("templates/layout.html", "templates/settings.html"),
		opencode: mustParse("templates/layout.html", "templates/opencode.html"),
		setup:    mustParse("templates/layout.html", "templates/setup.html"),
		assetsV:  assetsVersion(),
	}
}

func (r *renderer) render(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if pd, ok := data.(pageData); ok {
		pd.AssetsV = r.assetsV
		pd.AccentStyle = accentStyle(r.accentColor())
		if r.projectName != nil {
			pd.ProjectName = r.projectName()
		}
		data = pd
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// accentColor resolves through the wired hook, failing safe to the
// default accent (e.g. render tests that build a bare renderer).
func (r *renderer) accentColor() AccentColor {
	if r.accent == nil {
		return DefaultAccent()
	}
	return r.accent()
}

// accentStyle renders the head <style> overriding the accent CSS
// variables for both themes. Values are palette constants, so the
// template.HTML escape-out is safe. The id lets the settings picker
// live-preview a pending pick by rewriting the element's text.
func accentStyle(a AccentColor) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<style id="accent-style">:root{--accent:%[1]s;--accent-hover:%[2]s}[data-theme="light"]{--accent:%[3]s;--accent-hover:%[4]s}</style>`,
		a.Dark, a.DarkHover, a.Light, a.LightHover))
}

// --- view data ---

type pageData struct {
	Title string
	Page  string
	Data  any
	// ProjectName is the project display name rendered next to the logo
	// and as the tab title; filled centrally by render() through the
	// renderer hook.
	ProjectName string
	AssetsV     string
	// AccentStyle is the head <style> overriding the accent CSS
	// variables for the resolved palette; filled centrally by render().
	AccentStyle template.HTML
}

type indexView struct {
	Changes []changeSummary
	// GitRepo/GitDirty drive the Commit all button: hidden outside a git
	// repo, disabled when the tree is clean.
	GitRepo  bool
	GitDirty bool
	// OnboardingPending shows the finish-setup banner until onboarding is
	// completed or dismissed (.lessmess/onboarding.json).
	OnboardingPending bool
	// SpawnFallback, when non-nil, drives the spawn-fallback warning
	// banner (.lessmess/spawn-fallback.json).
	SpawnFallback *SpawnFallback
}

type tmplColumn struct {
	Status string
	Tasks  []cardView
}

type boardView struct {
	ID       string
	Overall  string
	Error    string
	Columns  []tmplColumn
	BoardURL string

	// Drill-down state (empty on the change's root board).
	Task      string      // viewed task ID
	NodeTitle string      // viewed task title
	Crumbs    []crumbView // ancestor chain, root change first

	// Worktree pipeline state; nil unless the change has a worktrees-state
	// entry (worktree-backed change).
	Worktree *worktreeView
}

// worktreeView is the board header's worktree strip: branch, path, health,
// PR, and review state.
type worktreeView struct {
	Branch string
	Path   string
	State  string // "active" | "dirty" | "missing"
	PRURL  string
	Review string // "", pending, done, failed
}

type crumbView struct {
	ID    string
	Title string
	URL   string
	Root  bool // the change itself
}

// tmplTaskRow is one card: the task's authoritative JSON state.
func tmplTaskRow(n *store.TaskNode) model.TaskState {
	if n.Task != nil {
		return *n.Task
	}
	return model.TaskState{ID: n.ID, Title: n.Href, Status: model.StatusNotStarted, File: n.Href}
}

// cardView is one kanban card: the task state plus the display-only
// subtree rollup badge data.
type cardView struct {
	model.TaskState
	Path     string // href minus the leading tasks/ (for the detail route)
	HasSub   bool
	SubDone  int
	SubTotal int
}

func cardOf(n *store.TaskNode) cardView {
	c := cardView{TaskState: tmplTaskRow(n), Path: strings.TrimPrefix(n.Href, "tasks/")}
	if n.HasContainer() {
		st := n.SubtreeStats()
		c.HasSub = true
		c.SubDone, c.SubTotal = st.Complete, st.Total
	}
	return c
}

// nodeRows renders a level's nodes as display rows.
func nodeRows(nodes []*store.TaskNode) []model.TaskState {
	out := make([]model.TaskState, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, tmplTaskRow(n))
	}
	return out
}

func newBoardView(c *store.Change) boardView {
	v := boardView{ID: c.ID, BoardURL: "/changes/" + c.ID}
	if c.Err != nil || c.State == nil {
		v.Error = "State unreadable: " + c.Err.Error()
		return v
	}
	v.Overall = string(c.Overall())
	v.Columns = columnsFromNodes(c.Roots)
	return v
}

// newTaskBoardView builds the drill-down board for one decomposed task:
// columns from its children, an ancestor breadcrumb, and the subtree
// rollup.
func newTaskBoardView(c *store.Change, n *store.TaskNode) boardView {
	v := boardView{
		ID: c.ID, BoardURL: "/changes/" + c.ID,
		Task: n.ID, NodeTitle: n.ID,
	}
	if n.Task != nil && n.Task.Title != "" {
		v.NodeTitle = n.Task.Title
	}
	v.Overall = string(c.Overall())
	v.Columns = columnsFromNodes(n.Children)
	// Breadcrumb: change root, then every ancestor, then this node.
	v.Crumbs = append(v.Crumbs, crumbView{ID: c.ID, Title: c.ID, URL: "/changes/" + c.ID, Root: true})
	var chain []*store.TaskNode
	for p := n.Parent; p != nil; p = p.Parent {
		chain = append([]*store.TaskNode{p}, chain...)
	}
	for _, p := range chain {
		title := p.ID
		if p.Task != nil && p.Task.Title != "" {
			title = p.Task.Title
		}
		v.Crumbs = append(v.Crumbs, crumbView{ID: p.ID, Title: title, URL: "/changes/" + c.ID + "?task=" + p.ID})
	}
	return v
}

func columnsFromNodes(nodes []*store.TaskNode) []tmplColumn {
	var cols []tmplColumn
	for _, st := range model.TaskStatusOrder {
		col := tmplColumn{Status: string(st)}
		for _, n := range nodes {
			if n.NodeStatus() == st {
				col.Tasks = append(col.Tasks, cardOf(n))
			}
		}
		cols = append(cols, col)
	}
	return cols
}

type taskView struct {
	Task     *model.TaskFile
	Change   string
	Status   string
	Statuses []string
	HasSub   bool
	SubDone  int
	SubTotal int
	Body     string
	Doc      string // change-relative href, for in-modal link resolution
}

type planView struct {
	ID   string
	Body string
	Doc  string
}

type ledgerView struct {
	ID   string
	Body string
	Doc  string // change-relative container href for container ledgers
}

// --- static assets ---

func staticHandler() (http.Handler, error) {
	sub, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, err
	}
	fileServer := http.StripPrefix("/static/", http.FileServerFS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vendored assets are pinned; app assets ship inside the binary.
		w.Header().Set("Cache-Control", "public, max-age=3600, immutable")
		fileServer.ServeHTTP(w, r)
	}), nil
}
