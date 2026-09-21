// Command lessmess is a kanban server for the changes/ workflow.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/opencode"
	"lessmess/internal/server"
	"lessmess/internal/store"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "init":
		return runInit(args[1:])
	case "migrate":
		return runMigrate(args[1:])
	case "docs":
		return runDocs(args[1:])
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `lessmess — kanban server for the changes/ workflow

Usage:
  lessmess serve    [--host 127.0.0.1] [--port 8080] [--dir .]
  lessmess validate [--dir .]
  lessmess migrate  [--dry-run] [--dir .]
  lessmess init     [--dir .]
  lessmess docs seed [--dry-run] [--budget N] [--dir .]
`)
}

type config struct {
	host string
	port int
	dir  string
}

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	var c config
	fs.StringVar(&c.host, "host", "127.0.0.1", "address to bind")
	fs.IntVar(&c.port, "port", 8080, "port to listen on")
	fs.StringVar(&c.dir, "dir", ".", "repository root containing changes/")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := store.MigrateStateDir(c.dir); err != nil {
		slog.Warn("state dir migration skipped", "err", err)
	}
	// Legacy markdown workflow state migrates to JSON before the store
	// opens (worktree-backed changes resolve through the same resolver).
	if _, err := store.MigrateIfNeeded(c.dir, server.WorktreeChangeRoot(c.dir)); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var cleanups []func()
	defer func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}()

	// boot builds the full handler for an initialized repository. It is the
	// single store-open path: used directly for a normal start, and by the
	// setup-mode server for its hot-open swap after bootstrap.
	boot := func(dir string) (http.Handler, error) {
		st, err := store.Open(dir)
		if err != nil {
			return nil, err
		}
		if err := st.Watch(ctx); err != nil {
			st.Close()
			return nil, err
		}
		cleanups = append(cleanups, st.Close)
		logStoreSummary(st)
		app := server.New(st)
		app.PublicBase = net.JoinHostPort(c.host, fmt.Sprint(c.port))
		if oc, err := opencode.DiscoverClient(ctx); err != nil {
			slog.Warn("opencode integration disabled", "err", err)
		} else {
			slog.Info("opencode service connected", "url", oc.BaseURL())
			app.SetOpencode(oc)
		}
		cleanups = append(cleanups, app.Close)
		return app.Handler(), nil
	}

	handler, err := boot(c.dir)
	if err != nil {
		if !errors.Is(err, store.ErrNoChanges) {
			slog.Error("open store", "err", err)
			return 1
		}
		slog.Info("no changes/ tree; starting in setup mode", "dir", c.dir)
		handler = server.NewSetup(c.dir, boot)
	}

	srv := &http.Server{
		Addr:              net.JoinHostPort(c.host, fmt.Sprint(c.port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("serving", "addr", srv.Addr, "dir", c.dir)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Error("shutdown", "err", err)
			return 1
		}
		return 0
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("serve", "err", err)
			return 1
		}
		return 0
	}
}

// logStoreSummary logs the scan and validation results of a freshly opened
// store (normal start and setup-mode hot-open alike).
func logStoreSummary(st *store.Store) {
	tasks := 0
	for _, ch := range st.Changes() {
		if ch.State != nil {
			tasks += len(ch.State.Tasks)
		}
	}
	slog.Info("scanned", "changes", len(st.Changes()), "tasks", tasks)
	if v := st.Validate(); len(v) > 0 {
		slog.Warn("validation violations found", "count", len(v))
		for _, vv := range v {
			slog.Warn("violation", "file", vv.File, "rule", vv.Rule, "msg", vv.Msg)
		}
	} else {
		slog.Info("validation ok")
	}
}

// runMigrate converts a repository's markdown workflow state into the
// JSON state store (one-way; see JSI-01). Worktree-backed changes
// resolve through the same resolver the server uses.
func runMigrate(args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository root containing changes/")
	dry := fs.Bool("dry-run", false, "verify and report without writing or deleting anything")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := store.MigrateStateDir(*dir); err != nil {
		slog.Warn("state dir migration skipped", "err", err)
	}
	res, err := store.MigrateWorkflow(store.MigrateOptions{
		Dir:        *dir,
		ChangeRoot: server.WorktreeChangeRoot(*dir),
		DryRun:     *dry,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}
	mode := "migrated"
	if *dry {
		mode = "verified (dry run)"
	}
	fmt.Printf("%s: %d changes, %d tasks, %d containers\n", mode, res.Changes, res.Tasks, res.Containers)
	if !*dry {
		fmt.Printf("rewrote %d task files, deleted %d ledger files, .gitignore updated\n", res.FilesRewritten, res.LedgersDeleted)
	}
	return 0
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", ".", "directory to bootstrap as a workflow repository")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	actions, err := docs.Init(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	for _, a := range actions {
		fmt.Printf("%-8s %s\n", a.Action, a.Path)
	}
	return 0
}

func runDocs(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "seed":
		return runDocsSeed(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown docs command %q\n\n", args[0])
		usage()
		return 2
	}
}

func runDocsSeed(args []string) int {
	fs := flag.NewFlagSet("docs seed", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository root")
	dry := fs.Bool("dry-run", false, "print the plan without writing or calling the LLM")
	budget := fs.Int("budget", 0, "max LLM sessions this run (0 = unlimited)")
	force := fs.Bool("force", false, "redo every covered directory, ignoring the resume cursor")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := docs.LoadConfig(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docs seed:", err)
		return 1
	}
	if cfg == nil {
		fmt.Fprintf(os.Stderr, "docs seed: %s has no %s — run 'lessmess init' first (docs system disabled)\n", *dir, docs.ConfigFile)
		return 1
	}
	// The opencode service requires an absolute session directory.
	root, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docs seed:", err)
		return 1
	}
	if err := store.MigrateStateDir(root); err != nil {
		slog.Warn("state dir migration skipped", "err", err)
	}
	var sum docs.Summarizer
	if !*dry {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		oc, err := opencode.DiscoverClient(ctx)
		cancel()
		if err != nil {
			slog.Warn("opencode service unavailable; writing skeletons only", "err", err)
		} else {
			agent, model := server.SessionDefaults(root)
			sum = docs.NewOpenCodeSummarizerWith(oc, 0, agent, model)
		}
	}
	if err := docs.Seed(context.Background(), root, cfg, sum, docs.SeedOptions{DryRun: *dry, Budget: *budget, Force: *force}, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "docs seed:", err)
		return 1
	}
	return 0
}

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository root containing changes/")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := store.MigrateStateDir(*dir); err != nil {
		slog.Warn("state dir migration skipped", "err", err)
	}
	if _, err := store.MigrateIfNeeded(*dir, server.WorktreeChangeRoot(*dir)); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}
	st, err := store.Open(*dir)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer st.Close()
	// Worktree-backed changes resolve their docs through the worktree, so
	// validation sees them where they actually live.
	st.SetChangeRoot(server.WorktreeChangeRoot(*dir))
	violations := st.Validate()
	findings := docs.ValidateDocs(*dir, queueStale(*dir))
	rc := 0
	if len(violations) == 0 && len(findings) == 0 {
		fmt.Println("OK")
		return 0
	}
	for _, v := range violations {
		fmt.Println(v.String())
		rc = 1
	}
	for _, f := range findings {
		fmt.Printf("%s: docs %s: %s\n", f.File, f.Severity, f.Msg)
		if f.Severity == docs.SeverityError {
			rc = 1
		}
	}
	return rc
}

// queueStale peeks at the server-owned docs queue file for stale flags. The
// schema is owned by internal/server; only the stale field is read.
func queueStale(dir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(dir, store.StateDirName, "docs-queue.json"))
	if err != nil {
		return nil
	}
	var q struct {
		Stale map[string]string `json:"stale"`
	}
	_ = json.Unmarshal(data, &q)
	return q.Stale
}
