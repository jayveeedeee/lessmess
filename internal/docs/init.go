package docs

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"lessmess/internal/model"
)

// workflowAgents is the canonical change-management instruction set that
// Init writes to a repository's root AGENTS.md. It is an exact copy of this
// repository's own AGENTS.md; a drift test enforces the sync.
//
//go:embed assets/workflow_agents.md
var workflowAgents string

// WorkflowInstructions returns the embedded canonical workflow text.
func WorkflowInstructions() string { return workflowAgents }

// rootLedgerSkeleton is the empty root ledger written by Init, matching the
// AGENTS.md root-ledger schema (including the required header sentence).
const rootLedgerSkeleton = `# Changes — Root Ledger

One row per change directory. This file is authoritative for change existence, task-ID prefixes, and overall change status only. Task statuses live exclusively in each change's ` + "`ledger.md`" + `.

| Change | Title | ID prefix | Branch | Status | Created | Last updated |
| --- | --- | --- | --- | --- | --- | --- |
`

// opencodeStarter is the starter agent-permission envelope written by Init:
// broad allow inside the project, denies for external directories, env
// files, and git push.
const opencodeStarter = `{
  "$schema": "https://opencode.ai/config.json",
  "permissions": [
    { "action": "*", "resource": "*", "effect": "allow" },
    { "action": "external_directory", "resource": "*", "effect": "deny" },
    { "action": "read", "resource": "*.env", "effect": "deny" },
    { "action": "read", "resource": "*.env.*", "effect": "deny" },
    { "action": "read", "resource": "*.env.example", "effect": "allow" },
    { "action": "shell", "resource": "git push *", "effect": "deny" }
  ]
}
`

// InitAction describes one artifact decision made by Init.
type InitAction struct {
	Path   string // repo-relative path
	Action string // "created", "merged", or "skipped"
}

// InitOptions tunes InitWithOptions. Config controls whether the default
// agentsdocs.json coverage config is written; without it the docs system
// stays disabled (the setup wizard makes coverage a separate choice from
// the rest of the bootstrap). Exclude adds user-chosen exclusion patterns
// to the written config (on top of the built-in DefaultExclude).
type InitOptions struct {
	Config  bool
	Exclude []string
}

// Init bootstraps root as a workflow-ready repository: root AGENTS.md with
// the canonical change-management instructions, the changes/ skeleton,
// .gitignore covering .lessmess/, a starter opencode.json, and the default
// agentsdocs.json. Git is not assumed. Every artifact is merge-safe —
// existing content is never clobbered — so Init is idempotent.
func Init(root string) ([]InitAction, error) {
	return InitWithOptions(root, InitOptions{Config: true})
}

// InitWithOptions is Init with optional steps: when opts.Config is false
// the agentsdocs.json step is skipped entirely (no file is created and no
// action is reported for it). opts.Exclude only applies when the config is
// actually created — an existing agentsdocs.json is never modified.
func InitWithOptions(root string, opts InitOptions) ([]InitAction, error) {
	steps := []func(string) (InitAction, error){
		initAgents,
		initLedger,
		initGitignore,
		initOpencode,
	}
	if opts.Config {
		steps = append(steps, func(root string) (InitAction, error) {
			return initConfigWith(root, opts.Exclude)
		})
	}
	var actions []InitAction
	for _, step := range steps {
		a, err := step(root)
		if err != nil {
			return actions, err
		}
		actions = append(actions, a)
	}
	return actions, nil
}

// initAgents writes the canonical workflow text as AGENTS.md. A freshly
// created file ends with an empty marker section: without an append target
// the seed/gardener passes have nowhere sanctioned to write learnings, and
// the root dir can never complete seeding (the model must invent its own
// auto section and history shows it rewrites the workflow text instead —
// confinement correctly rolls that back, but the dir stays pending). An
// existing file that already carries the text is skipped — unless it has
// no marker section yet (repos bootstrapped before this convention), in
// which case one empty section is appended; any other existing file gets
// the text merged into a marker-guarded auto section (human content kept,
// refreshable by a later Init).
func initAgents(root string) (InitAction, error) {
	a := InitAction{Path: AgentsFile}
	p := filepath.Join(root, AgentsFile)
	existing, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		err = writeInitFile(p, rootAgentsWithSection(), &a, "created")
		return a, err
	}
	if err != nil {
		return a, err
	}
	if strings.Contains(string(existing), strings.TrimRight(workflowAgents, "\n")) {
		doc, perr := model.ParseDocFile(AgentsFile, existing)
		if perr == nil && !doc.HasAuto {
			merged := strings.TrimRight(string(existing), "\n") + "\n\n" +
				model.DocMarkerBegin + "\n" + model.DocMarkerEnd + "\n"
			err = writeInitFile(p, []byte(merged), &a, "merged")
			return a, err
		}
		a.Action = "skipped"
		return a, nil
	}
	merged, err := model.MergeDoc(AgentsFile, existing, []byte(workflowAgents))
	if err != nil {
		return a, err
	}
	err = writeInitFile(p, merged, &a, "merged")
	return a, err
}

// rootAgentsWithSection renders the workflow text with the empty auto
// section appended — the append target for seed and gardener passes.
func rootAgentsWithSection() []byte {
	return []byte(strings.TrimRight(workflowAgents, "\n") + "\n\n" +
		model.DocMarkerBegin + "\n" + model.DocMarkerEnd + "\n")
}

// initLedger creates the changes/ skeleton; an existing ledger is skipped.
func initLedger(root string) (InitAction, error) {
	a := InitAction{Path: "changes/ledger.md"}
	p := filepath.Join(root, "changes", "ledger.md")
	if _, err := os.Stat(p); err == nil {
		a.Action = "skipped"
		return a, nil
	} else if !os.IsNotExist(err) {
		return a, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return a, err
	}
	err := writeInitFile(p, []byte(rootLedgerSkeleton), &a, "created")
	return a, err
}

// initGitignore ensures .gitignore ignores .lessmess/. A missing file is
// created; an existing file gains one line; a file already covering it is
// skipped.
func initGitignore(root string) (InitAction, error) {
	a := InitAction{Path: ".gitignore"}
	p := filepath.Join(root, ".gitignore")
	existing, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		err = writeInitFile(p, []byte(".lessmess/\n"), &a, "created")
		return a, err
	}
	if err != nil {
		return a, err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == ".lessmess" || strings.TrimSpace(line) == ".lessmess/" {
			a.Action = "skipped"
			return a, nil
		}
	}
	merged := strings.TrimRight(string(existing), "\n") + "\n.lessmess/\n"
	err = writeInitFile(p, []byte(merged), &a, "merged")
	return a, err
}

// initOpencode writes the starter permission envelope; an existing file is
// skipped.
func initOpencode(root string) (InitAction, error) {
	a := InitAction{Path: "opencode.json"}
	err := writeIfAbsent(root, "opencode.json", []byte(opencodeStarter), &a)
	return a, err
}

// initConfig writes the default coverage config; an existing file is skipped.
func initConfig(root string) (InitAction, error) {
	return initConfigWith(root, nil)
}

// initConfigWith is initConfig carrying user-chosen exclusion patterns.
func initConfigWith(root string, exclude []string) (InitAction, error) {
	a := InitAction{Path: ConfigFile}
	cfg := DefaultConfig()
	cfg.Exclude = append(cfg.Exclude, exclude...)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return a, err
	}
	err = writeIfAbsent(root, ConfigFile, append(data, '\n'), &a)
	return a, err
}

func writeIfAbsent(root, rel string, data []byte, a *InitAction) error {
	p := filepath.Join(root, rel)
	if _, err := os.Stat(p); err == nil {
		a.Action = "skipped"
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeInitFile(p, data, a, "created")
}

func writeInitFile(path string, data []byte, a *InitAction, action string) error {
	if err := model.WriteFileAtomic(path, data, 0o644); err != nil {
		return err
	}
	a.Action = action
	return nil
}
