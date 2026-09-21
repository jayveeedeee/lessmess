package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/opencode"
	"lessmess/internal/store"
)

// ocCapture is a fake opencode service recording create bodies and prompt
// texts. The reject flags answer 400 for creates carrying the respective
// key: rejectDefaults covers agent and model, rejectAgent/rejectModel are
// finer-grained for ladder tests. agentStatus (when non-zero) overrides the
// status for agent-carrying creates, simulating non-400 failures.
type ocCapture struct {
	creates        []map[string]any
	prompts        []string
	deletes        int
	failPrompts    bool
	rejectDefaults bool
	rejectAgent    bool
	rejectModel    bool
	agentStatus    int
}

func (c *ocCapture) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			c.deletes++
			w.Write([]byte(`{"data":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			c.creates = append(c.creates, body)
			if _, ok := body["agent"]; ok && (c.rejectDefaults || c.rejectAgent || c.agentStatus != 0) {
				code := http.StatusBadRequest
				if c.agentStatus != 0 {
					code = c.agentStatus
				}
				w.WriteHeader(code)
				w.Write([]byte(`{"message":"unknown agent"}`))
				return
			}
			if _, ok := body["model"]; ok && (c.rejectDefaults || c.rejectModel) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"message":"unknown model"}`))
				return
			}
			w.Write([]byte(`{"data":{"id":"ses_cap","title":"t","location":{"directory":"/x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			c.prompts = append(c.prompts, body["text"])
			if c.failPrompts {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"message":"prompt boom"}`))
				return
			}
			w.Write([]byte(`{"data":{}}`))
		default:
			w.Write([]byte(`{"data":{}}`))
		}
	}
}

func writeSettingsFile(t *testing.T, dir string, s Settings) {
	t.Helper()
	writeJSONFile(t, settingsProjectPath(dir), s)
}

func TestSpawnSessionAppliesAgentAndModel(t *testing.T) {
	cap := &ocCapture{}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{
		Agent: "build",
		Model: "prov/accounts/x/model-1",
	}})

	w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	if len(cap.creates) != 1 {
		t.Fatalf("creates = %d", len(cap.creates))
	}
	body := cap.creates[0]
	if body["agent"] != "build" {
		t.Errorf("agent = %v, want build", body["agent"])
	}
	m, _ := body["model"].(map[string]any)
	if m["providerID"] != "prov" || m["id"] != "accounts/x/model-1" {
		t.Errorf("model = %v, want prov + accounts/x/model-1 (split on first /)", body["model"])
	}
}

func TestSpawnSessionDefaultsOmittedWhenUnset(t *testing.T) {
	cap := &ocCapture{}
	s := mappingServer(t, cap.handler())

	w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	if _, ok := cap.creates[0]["agent"]; ok {
		t.Errorf("agent key must be absent without settings: %v", cap.creates[0])
	}
	if _, ok := cap.creates[0]["model"]; ok {
		t.Errorf("model key must be absent without settings: %v", cap.creates[0])
	}
}

func TestSpawnSessionFallbackOn400(t *testing.T) {
	cap := &ocCapture{rejectDefaults: true}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: "ghost", Model: "gone/m"}})

	w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s — fallback must still yield a session", w.Code, w.Body)
	}
	// Ladder: agent+model, agent-only, model-only, plain.
	if len(cap.creates) != 4 {
		t.Fatalf("creates = %d, want 4 (full de-escalation ladder)", len(cap.creates))
	}
	for _, want := range []struct {
		idx int
		key string
	}{
		{0, "agent"}, {0, "model"}, {1, "agent"}, {2, "model"},
	} {
		if _, ok := cap.creates[want.idx][want.key]; !ok {
			t.Errorf("create %d missing %s: %v", want.idx, want.key, cap.creates[want.idx])
		}
	}
	if _, ok := cap.creates[3]["agent"]; ok {
		t.Errorf("plain retry must drop the configured agent: %v", cap.creates[3])
	}
	if _, ok := cap.creates[3]["model"]; ok {
		t.Errorf("plain retry must drop the configured model: %v", cap.creates[3])
	}
}

func TestPromptAddendaAppended(t *testing.T) {
	cap := &ocCapture{}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Prompts: PromptSettings{
		Discussion: "DISCUSSION-ADDENDUM",
		Change:     "CHANGE-ADDENDUM",
	}})

	if w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`); w.Code != 201 {
		t.Fatalf("discussion: %d %s", w.Code, w.Body)
	}
	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/sessions", `{}`); w.Code != 201 {
		t.Fatalf("change session: %d %s", w.Code, w.Body)
	}
	if len(cap.prompts) != 2 {
		t.Fatalf("prompts = %d", len(cap.prompts))
	}
	if !strings.Contains(cap.prompts[0], "planning and execution assistant") || !strings.HasSuffix(cap.prompts[0], "\n\nDISCUSSION-ADDENDUM") {
		t.Errorf("discussion prompt missing base or addendum: %q…", cap.prompts[0][:80])
	}
	if !strings.Contains(cap.prompts[1], "change execution assistant") || !strings.HasSuffix(cap.prompts[1], "\n\nCHANGE-ADDENDUM") {
		t.Errorf("change prompt missing base or addendum: %q…", cap.prompts[1][:80])
	}
}

func TestPromptByteIdenticalWithoutSettings(t *testing.T) {
	cap := &ocCapture{}
	s := mappingServer(t, cap.handler())

	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/sessions", `{}`); w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	want, _ := s.changePrime("2026-09-10-0", "ses_cap")
	if len(cap.prompts) != 1 || cap.prompts[0] != want {
		t.Error("prompt must be byte-identical to the base builder without settings")
	}
}

func TestDefaultBranchRecorded(t *testing.T) {
	cap := &ocCapture{}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Git: GitSettings{DefaultBranch: "main"}})

	// Plain create route.
	w := do(t, s.Handler(), "POST", "/changes/", `{"title":"Branched","prefix":"BRA"}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	idx, err := s.st.Index()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range idx.Changes {
		if e.Title == "Branched" {
			found = true
			if e.Branch != "main" {
				t.Errorf("branch = %q, want main", e.Branch)
			}
		}
	}
	if !found {
		t.Fatal("created change missing from index")
	}

	// Scaffold route shares the same settings path.
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_br", Title: "d", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"Scaffolded","prefix":"SCA","session":"ses_br"}`)
	if w.Code != 201 {
		t.Fatalf("scaffold: %d %s", w.Code, w.Body)
	}
	idx, _ = s.st.Index()
	for _, e := range idx.Changes {
		if e.Title == "Scaffolded" && e.Branch != "main" {
			t.Errorf("scaffold branch = %q, want main", e.Branch)
		}
	}
}

func TestGardenerGateOnClose(t *testing.T) {
	s, dir := docsServer(t)
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "00-first.md", "- internal/model/docfile.go\n")
	writeSettingsFile(t, dir, Settings{Docs: DocsSettings{AutoGardenerOnClose: boolp(false)}})
	runner := &fakeRunner{}
	s.SetDocsRunner(runner)

	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", ""); w.Code != 200 {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	// Give the (non-existent) enqueue a beat: nothing may be enqueued.
	time.Sleep(150 * time.Millisecond)
	if got := len(runner.got()); got != 0 {
		t.Fatalf("gardener ran %d jobs with the gate off", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lessmess", "docs-queue.json")); !os.IsNotExist(err) {
		t.Error("queue file must not exist when the gate is off")
	}

	// Reopen, enable the gate, close again: the job runs.
	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/reopen", ""); w.Code != 200 {
		t.Fatalf("reopen: %d %s", w.Code, w.Body)
	}
	writeSettingsFile(t, dir, Settings{})
	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", ""); w.Code != 200 {
		t.Fatalf("second close: %d %s", w.Code, w.Body)
	}
	waitForCond(t, "docs job after enabling gate", func() bool { return len(runner.got()) == 1 })
}

func TestIndexArchivedFilter(t *testing.T) {
	s := mappingServer(t, nil)
	// Register an archived change (index entry + archive prose dir) and reload.
	archDir := filepath.Join(s.st.Dir, "changes", "archive", "2026-09-08-0")
	if err := os.MkdirAll(filepath.Join(archDir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archDir, "plan.md"), model.RenderChangePlan("2026-09-08-0", "Old archived", "2026-09-08"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd := filepath.Join(s.st.Dir, store.StateDirName, "workflow")
	archState := &model.ChangeState{
		Version: model.StateVersion, ID: "2026-09-08-0", Title: "Old archived", Prefix: "OLD",
		Status: model.ChangeStatus{Value: model.OverallDone}, Created: "2026-09-08", Updated: "2026-09-08",
	}
	if err := archState.Save(filepath.Join(wd, "changes", "2026-09-08-0.json")); err != nil {
		t.Fatal(err)
	}
	idx, err := model.LoadWorkflowIndex(filepath.Join(wd, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	idx.Changes = append(idx.Changes, model.IndexEntry{ID: "2026-09-08-0", Title: "Old archived", Prefix: "OLD", Created: "2026-09-08", Archived: true})
	if err := idx.Save(filepath.Join(wd, "index.json")); err != nil {
		t.Fatal(err)
	}
	s.st.Reload()

	indexIDs := func() []string {
		w := do(t, s.Handler(), "GET", "/", "")
		if w.Code != 200 {
			t.Fatalf("index: %d", w.Code)
		}
		var resp struct {
			Changes []changeSummary `json:"changes"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		ids := make([]string, 0, len(resp.Changes))
		for _, c := range resp.Changes {
			ids = append(ids, c.ID)
		}
		return ids
	}

	// Default: archived rows are listed (current behavior).
	ids := indexIDs()
	if len(ids) != 2 {
		t.Fatalf("default index ids = %v, want 2 rows", ids)
	}

	writeSettingsFile(t, s.st.Dir, Settings{UI: UISettings{ShowArchived: boolp(false)}})
	ids = indexIDs()
	if len(ids) != 1 || ids[0] != "2026-09-10-0" {
		t.Fatalf("filtered index ids = %v, want only the active change", ids)
	}
}

func TestGardenerRunnerSpawnAndAddendum(t *testing.T) {
	fake := &fakeSessionClient{}
	root, cfg := gardenerRepo(t)

	// With a settings addendum and no spawn: plain create, addendum appended.
	writeSettingsFile(t, root, Settings{Prompts: PromptSettings{Gardener: "GARDENER-ADDENDUM"}})
	r := &gardenerRunner{oc: fake, root: root, cfg: cfg}
	if err := r.garden(context.Background(), DocsJob{Change: "manual", Title: "manual"}, nil, nil); err != nil {
		t.Fatalf("garden: %v", err)
	}
	if len(fake.prompts) != 1 || !strings.HasSuffix(fake.prompts[0], "\n\nGARDENER-ADDENDUM") {
		t.Fatalf("gardener prompt missing addendum: %v", fake.prompts)
	}

	// With spawn wired (as SetOpencode does), spawn replaces plain creation.
	spawned := 0
	r.spawn = func(_ context.Context, title string) (*opencode.Session, error) {
		spawned++
		return &opencode.Session{ID: "ses_spawn", Title: title}, nil
	}
	if err := r.garden(context.Background(), DocsJob{Change: "manual", Title: "manual"}, nil, nil); err != nil {
		t.Fatalf("garden with spawn: %v", err)
	}
	if spawned != 1 {
		t.Errorf("spawn called %d times, want 1", spawned)
	}
}

func TestGardenerSpawnUsesGardenerModel(t *testing.T) {
	cap := &ocCapture{}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{
		Session: SessionSettings{Agent: "build", Model: "prov/session-m"},
		Docs:    DocsSettings{GardenerModel: "prov/gardener-m"},
	})

	// The gardener closure resolves docs.gardenerModel over session.model.
	if _, err := spawnSessionWithModel(context.Background(), s.oc, s.st.Dir, s.st.Dir, "t — docs", GardenerModel(s.st.Dir)); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if len(cap.creates) != 1 {
		t.Fatalf("creates = %d", len(cap.creates))
	}
	body := cap.creates[0]
	if body["agent"] != "build" {
		t.Errorf("agent = %v, want the session agent (override is model-only)", body["agent"])
	}
	m, _ := body["model"].(map[string]any)
	if m["providerID"] != "prov" || m["id"] != "gardener-m" {
		t.Errorf("model = %v, want the gardener override", body["model"])
	}

	// Without the override, the closure falls back to the session model.
	if _, err := spawnSessionWithModel(context.Background(), s.oc, s.st.Dir, s.st.Dir, "t — docs", ""); err != nil {
		t.Fatalf("spawn without override: %v", err)
	}
	m, _ = cap.creates[1]["model"].(map[string]any)
	if m["providerID"] != "prov" || m["id"] != "session-m" {
		t.Errorf("fallback model = %v, want the session model", cap.creates[1]["model"])
	}
}
