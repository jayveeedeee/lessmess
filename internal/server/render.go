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

	"lessmess/internal/model"
	"lessmess/internal/store"
	"lessmess/web"
)

// --- templates ---

var md = goldmark.New()

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
	"base":     filepath.Base,
	"markdown": renderMarkdown,
}

type renderer struct {
	index    *template.Template
	board    *template.Template
	partial  *template.Template
	explorer *template.Template
	settings *template.Template
	assetsV  string
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
	for _, name := range []string{"static/app.css", "static/app.js", "static/xterm.min.css", "static/xterm.min.js", "static/xterm-addon-fit.min.js"} {
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
		assetsV:  assetsVersion(),
	}
}

func (r *renderer) render(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if pd, ok := data.(pageData); ok {
		pd.AssetsV = r.assetsV
		data = pd
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// --- view data ---

type pageData struct {
	Title   string
	Page    string
	Data    any
	AssetsV string
}

type indexView struct {
	Changes []changeSummary
	// GitRepo/GitDirty drive the Commit all button: hidden outside a git
	// repo, disabled when the tree is clean.
	GitRepo  bool
	GitDirty bool
}

type tmplColumn struct {
	Status string
	Tasks  []model.TaskRow
}

type boardView struct {
	ID       string
	Overall  string
	Error    string
	Columns  []tmplColumn
	BoardURL string
}

func newBoardView(c *store.Change) boardView {
	v := boardView{ID: c.ID, BoardURL: "/changes/" + c.ID}
	if c.Err != nil || c.Ledger == nil {
		v.Error = "Ledger unreadable: " + c.Err.Error()
		return v
	}
	v.Overall = string(c.Ledger.Overall)
	for _, st := range model.TaskStatusOrder {
		col := tmplColumn{Status: string(st)}
		for _, t := range c.Ledger.Rows {
			if t.Status == st {
				col.Tasks = append(col.Tasks, t)
			}
		}
		v.Columns = append(v.Columns, col)
	}
	return v
}

type taskView struct {
	Task   *model.TaskFile
	Status string
	Body   string
}

type planView struct {
	ID   string
	Body string
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
