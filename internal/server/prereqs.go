package server

// Prerequisite checks for the setup wizard: one probe per thing the
// wizard (and later the full UI) depends on. Each check reports
// ok/warn/fail with a human detail line and a remediation hint; the wizard
// offers a Re-check button that simply re-fetches this endpoint. The
// checks never try to fix anything themselves (no auto-starting services).
// Probes go through package-level function vars so tests can fake them
// (same pattern as Server.SpawnCommand).

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/opencode"
)

// prereqCheck is one probe result. Status is "ok", "warn", or "fail";
// "fail" blocks the wizard's continue gate (Ready), "warn" does not.
type prereqCheck struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Remedy string `json:"remedy,omitempty"`
}

// prereqsResponse is the payload of GET /api/setup/prereqs.
type prereqsResponse struct {
	Checks []prereqCheck `json:"checks"`
	Ready  bool          `json:"ready"`
}

// --- injectable probes ---

// prereqLookPath finds a binary on PATH.
var prereqLookPath = exec.LookPath

// prereqProbeService probes the opencode background service in stages and
// returns an authenticated client plus its base URL. On failure the stage
// is "service" (status command), "credentials" (service.json), or "health"
// (authenticated health check) so the check can pick a remedy.
var prereqProbeService = func(ctx context.Context) (c *opencode.Client, base, stage string, err error) {
	base, err = opencode.Discover(ctx)
	if err != nil {
		return nil, "", "service", err
	}
	pw, err := opencode.PasswordFromFile(opencode.DefaultPasswordPath())
	if err != nil {
		return nil, base, "credentials", err
	}
	c = opencode.New(base, pw)
	if err := c.Healthy(ctx); err != nil {
		return nil, base, "health", err
	}
	return c, base, "", nil
}

// prereqProbeGit reports (git binary present, dir inside a work tree).
var prereqProbeGit = func(ctx context.Context, dir string) (binOK, repoOK bool) {
	if _, err := prereqLookPath("git"); err != nil {
		return false, false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err != nil {
		return true, false
	}
	return true, true
}

// prereqWritable verifies the repository directory is writable by creating
// and removing a temp file in it. The .tt- prefix is ignored by the store
// and docs watchers.
var prereqWritable = func(dir string) error {
	f, err := os.CreateTemp(dir, ".tt-prereq-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

// prereqs handles GET /api/setup/prereqs: run all probes and report. On a
// successful service probe the discovered client is kept (setup mode had
// none at startup) so the settings options/validation endpoints can use it.
func (env *setupEnv) prereqs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, env.runPrereqs(ctx))
}

func (env *setupEnv) runPrereqs(ctx context.Context) prereqsResponse {
	resp := prereqsResponse{Checks: []prereqCheck{}}
	add := func(c prereqCheck) { resp.Checks = append(resp.Checks, c) }

	// 1. opencode2 binary.
	binPath, binErr := prereqLookPath("opencode2")
	if binErr == nil {
		add(prereqCheck{ID: "opencode2-binary", Name: "opencode2 binary", Status: "ok", Detail: "found at " + binPath})
	} else {
		add(prereqCheck{ID: "opencode2-binary", Name: "opencode2 binary", Status: "fail",
			Detail: "opencode2 was not found on PATH",
			Remedy: "Install opencode2 and make sure it is on PATH, then Re-check."})
	}

	// 2. opencode service (only probeable with the binary).
	if binErr != nil {
		add(prereqCheck{ID: "opencode-service", Name: "opencode service", Status: "fail",
			Detail: "skipped: the opencode2 binary is missing",
			Remedy: "Install opencode2 first, then Re-check."})
	} else if c, base, stage, err := prereqProbeService(ctx); err != nil {
		chk := prereqCheck{ID: "opencode-service", Name: "opencode service", Status: "fail"}
		switch stage {
		case "service":
			chk.Detail = "the background service is not running (opencode2 service status failed)"
			chk.Remedy = "Start the opencode background service, then Re-check."
		case "credentials":
			chk.Detail = fmt.Sprintf("credentials unreadable: %v", err)
			chk.Remedy = "Check ~/.config/opencode/service.json (starting the service creates it), then Re-check."
		default:
			chk.Detail = fmt.Sprintf("service at %s failed the authenticated health check", base)
			chk.Remedy = "Verify the running service matches ~/.config/opencode/service.json, then Re-check."
		}
		add(chk)
	} else {
		add(prereqCheck{ID: "opencode-service", Name: "opencode service", Status: "ok", Detail: "connected to " + base})
		if env.setOC != nil {
			env.setOC(c)
		}
	}

	// 3. git binary + work tree (soft: commit features degrade gracefully).
	binOK, repoOK := prereqProbeGit(ctx, env.dir)
	if !binOK {
		add(prereqCheck{ID: "git-binary", Name: "git binary", Status: "warn",
			Detail: "git was not found on PATH",
			Remedy: "Install git if you want the commit features."})
		add(prereqCheck{ID: "git-repo", Name: "git repository", Status: "warn",
			Detail: "skipped: the git binary is missing"})
	} else if !repoOK {
		add(prereqCheck{ID: "git-binary", Name: "git binary", Status: "ok", Detail: "found on PATH"})
		add(prereqCheck{ID: "git-repo", Name: "git repository", Status: "warn",
			Detail: env.dir + " is not inside a git work tree",
			Remedy: "Run git init if you want the commit features; everything else works without git."})
	} else {
		add(prereqCheck{ID: "git-binary", Name: "git binary", Status: "ok", Detail: "found on PATH"})
		add(prereqCheck{ID: "git-repo", Name: "git repository", Status: "ok", Detail: "inside a git work tree"})
	}

	// 4. repository directory writable.
	if err := prereqWritable(env.dir); err != nil {
		add(prereqCheck{ID: "repo-writable", Name: "repository writable", Status: "fail",
			Detail: fmt.Sprintf("cannot write in %s: %v", env.dir, err),
			Remedy: "Fix the permissions on the repository directory, then Re-check."})
	} else {
		add(prereqCheck{ID: "repo-writable", Name: "repository writable", Status: "ok", Detail: env.dir + " is writable"})
	}

	// 5. changes/ tree present (informational: drives where the wizard resumes).
	// A present-but-incomplete tree (no root ledger) is bootstrappable, just
	// like a missing one — only a corrupt ledger is fatal, and serve reports
	// that before the wizard starts.
	changesDir := filepath.Join(env.dir, "changes")
	if st, err := os.Stat(changesDir); err == nil && st.IsDir() {
		if _, err := os.Stat(filepath.Join(changesDir, "ledger.md")); err == nil {
			add(prereqCheck{ID: "changes-present", Name: "changes/ tree", Status: "ok", Detail: "changes/ exists"})
		} else {
			add(prereqCheck{ID: "changes-present", Name: "changes/ tree", Status: "warn",
				Detail: "changes/ exists but has no root ledger; the bootstrap step creates it"})
		}
	} else {
		add(prereqCheck{ID: "changes-present", Name: "changes/ tree", Status: "warn",
			Detail: "changes/ does not exist yet; the bootstrap step creates it"})
	}

	// 6. docs coverage config (informational: drives the docs step's visibility).
	if cfg, err := docs.LoadConfig(env.dir); err == nil && cfg != nil {
		add(prereqCheck{ID: "docs-coverage", Name: "docs coverage", Status: "ok", Detail: "agentsdocs.json present; docs system enabled"})
	} else {
		add(prereqCheck{ID: "docs-coverage", Name: "docs coverage", Status: "warn",
			Detail: "no agentsdocs.json; the bootstrap step can enable coverage, or the docs step is skipped"})
	}

	resp.Ready = true
	for _, c := range resp.Checks {
		if c.Status == "fail" {
			resp.Ready = false
			break
		}
	}
	return resp
}
