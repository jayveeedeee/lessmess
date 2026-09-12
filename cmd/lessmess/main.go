// Command lessmess is a kanban server for the changes/ workflow.
package main

import (
	"context"
	"encoding/json"
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
	st, err := store.Open(c.dir)
	if err != nil {
		slog.Error("open store", "err", err)
		return 1
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := st.Watch(ctx); err != nil {
		slog.Error("watch", "err", err)
		return 1
	}
	if changes := st.Changes(); true {
		tasks := 0
		for _, ch := range changes {
			if ch.Ledger != nil {
				tasks += len(ch.Ledger.Rows)
			}
		}
		slog.Info("scanned", "changes", len(changes), "tasks", tasks)
	}
	if v := st.Validate(); len(v) > 0 {
		slog.Warn("validation violations found", "count", len(v))
		for _, vv := range v {
			slog.Warn("violation", "file", vv.File, "rule", vv.Rule, "msg", vv.Msg)
		}
	} else {
		slog.Info("validation ok")
	}

	app := server.New(st)
	app.PublicBase = net.JoinHostPort(c.host, fmt.Sprint(c.port))
	if oc, err := opencode.DiscoverClient(ctx); err != nil {
		slog.Warn("opencode integration disabled", "err", err)
	} else {
		slog.Info("opencode service connected", "url", oc.BaseURL())
		app.SetOpencode(oc)
	}
	srv := &http.Server{
		Addr:              net.JoinHostPort(c.host, fmt.Sprint(c.port)),
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	defer app.Close()

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
			sum = docs.NewOpenCodeSummarizer(oc, 0)
		}
	}
	if err := docs.Seed(context.Background(), root, cfg, sum, docs.SeedOptions{DryRun: *dry, Budget: *budget}, os.Stdout); err != nil {
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
	st, err := store.Open(*dir)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer st.Close()
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
