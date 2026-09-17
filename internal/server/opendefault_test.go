package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOpencodeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadOpencodeDefault(t *testing.T) {
	t.Run("absent file", func(t *testing.T) {
		got := readOpencodeDefault(t.TempDir())
		if got.Status != ocDefaultAbsent || got.Declared != "" {
			t.Errorf("got %+v, want absent", got)
		}
	})
	t.Run("declared", func(t *testing.T) {
		dir := t.TempDir()
		writeOpencodeConfig(t, dir, `{"default_agent":"kg-orchestrator","watcher":{"ignore":["a"]}}`+"\n")
		got := readOpencodeDefault(dir)
		if got.Status != ocDefaultOK || got.Declared != "kg-orchestrator" {
			t.Errorf("got %+v, want ok/kg-orchestrator", got)
		}
		if got.Path != "opencode.json" {
			t.Errorf("path = %q", got.Path)
		}
	})
	t.Run("no key", func(t *testing.T) {
		dir := t.TempDir()
		writeOpencodeConfig(t, dir, `{"watcher":{"ignore":["a"]}}`+"\n")
		if got := readOpencodeDefault(dir); got.Status != ocDefaultAbsent {
			t.Errorf("got %+v, want absent", got)
		}
	})
	t.Run("jsonc unreadable", func(t *testing.T) {
		dir := t.TempDir()
		writeOpencodeConfig(t, dir, "{\n  // comment\n  \"default_agent\": \"x\",\n}\n")
		if got := readOpencodeDefault(dir); got.Status != ocDefaultUnreadable {
			t.Errorf("got %+v, want unreadable", got)
		}
	})
}

func TestSettingsResponseCarriesOpencodeDefault(t *testing.T) {
	dir := t.TempDir()
	if resp := settingsAPIView(dir); resp.OpencodeDefaultAgent != nil {
		t.Errorf("absent config must leave the field nil, got %+v", resp.OpencodeDefaultAgent)
	}
	writeOpencodeConfig(t, dir, `{"default_agent":"kg-orchestrator"}`+"\n")
	resp := settingsAPIView(dir)
	if resp.OpencodeDefaultAgent == nil || resp.OpencodeDefaultAgent.Declared != "kg-orchestrator" {
		t.Fatalf("declared config must surface, got %+v", resp.OpencodeDefaultAgent)
	}
	// Round-trips through the response JSON shape the UI consumes.
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"opencodeDefaultAgent":`, `"declared":"kg-orchestrator"`, `"status":"ok"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("payload missing %s: %s", want, b)
		}
	}
}

func TestSettingsPageCarriesDefaultAgentHooks(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/settings", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`id="opencode-default-note"`,
		`id="opencode-default-text"`,
		`id="align-default-agent"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settings HTML missing %q", want)
		}
	}
}

func TestPatchDefaultAgent(t *testing.T) {
	t.Run("preserves everything but the value", func(t *testing.T) {
		in := "{\n  \"default_agent\": \"kg-orchestrator\",\n  \"watcher\": {\n    \"ignore\": [\n      \"a\"\n    ]\n  },\n  \"zz_last\": 1\n}\n"
		want := "{\n  \"default_agent\": \"build\",\n  \"watcher\": {\n    \"ignore\": [\n      \"a\"\n    ]\n  },\n  \"zz_last\": 1\n}\n"
		out, old, err := patchDefaultAgent([]byte(in), "build")
		if err != nil {
			t.Fatalf("patch: %v", err)
		}
		if old != "kg-orchestrator" {
			t.Errorf("old = %q", old)
		}
		if string(out) != want {
			t.Errorf("patched = %q, want %q", out, want)
		}
	})
	t.Run("no key", func(t *testing.T) {
		if _, _, err := patchDefaultAgent([]byte("{\"a\":1}\n"), "build"); err == nil {
			t.Error("expected an error without default_agent")
		}
	})
	t.Run("jsonc refused", func(t *testing.T) {
		if _, _, err := patchDefaultAgent([]byte("{ // c\n \"default_agent\": \"x\"\n}\n"), "build"); err == nil {
			t.Error("JSONC must be refused, never rewritten")
		}
	})
	t.Run("not an object", func(t *testing.T) {
		if _, _, err := patchDefaultAgent([]byte("[1]\n"), "build"); err == nil {
			t.Error("expected an error for a non-object root")
		}
	})
}

// agentServe serves the /api/agent list (build primary, general subagent)
// plus a catch-all; agentStatus makes /api/agent fail with that status,
// simulating an unreachable service.
func agentServe(agentStatus int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agent" {
			if agentStatus != 0 {
				w.WriteHeader(agentStatus)
				w.Write([]byte(`{"message":"down"}`))
				return
			}
			w.Write([]byte(`{"data":[{"id":"build","name":"Build","mode":"primary","hidden":false},{"id":"general","name":"General","mode":"subagent","hidden":false}]}`))
			return
		}
		w.Write([]byte(`{"data":{}}`))
	}
}

func TestAlignOpencodeDefault(t *testing.T) {
	newAlignServer := func(t *testing.T, agent string, agentStatus int) *Server {
		s := mappingServer(t, agentServe(agentStatus))
		writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: agent}})
		writeOpencodeConfig(t, s.st.Dir, "{\"default_agent\":\"kg-orchestrator\",\"watcher\":{\"ignore\":[\"x\"]}}\n")
		return s
	}

	t.Run("aligns and preserves the rest", func(t *testing.T) {
		s := newAlignServer(t, "build", 0)
		w := do(t, s.Handler(), "POST", "/api/settings/opencode-default-agent", "")
		if w.Code != 200 {
			t.Fatalf("code = %d body = %s", w.Code, w.Body)
		}
		got, err := os.ReadFile(filepath.Join(s.st.Dir, "opencode.json"))
		if err != nil {
			t.Fatal(err)
		}
		if want := "{\"default_agent\":\"build\",\"watcher\":{\"ignore\":[\"x\"]}}\n"; string(got) != want {
			t.Errorf("file = %q, want %q", got, want)
		}
		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["old"] != "kg-orchestrator" || resp["new"] != "build" {
			t.Errorf("resp = %v", resp)
		}
		// The divergence is gone: the response no longer surfaces it.
		if resp2 := settingsAPIView(s.st.Dir); resp2.OpencodeDefaultAgent != nil && resp2.OpencodeDefaultAgent.Declared != "build" {
			t.Errorf("declared = %q, want build", resp2.OpencodeDefaultAgent.Declared)
		}
	})

	t.Run("unknown agent 422 and file untouched", func(t *testing.T) {
		s := newAlignServer(t, "ghost", 0)
		before, _ := os.ReadFile(filepath.Join(s.st.Dir, "opencode.json"))
		w := do(t, s.Handler(), "POST", "/api/settings/opencode-default-agent", "")
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("code = %d body = %s", w.Code, w.Body)
		}
		after, _ := os.ReadFile(filepath.Join(s.st.Dir, "opencode.json"))
		if string(before) != string(after) {
			t.Error("a refused align must leave the file untouched")
		}
	})

	t.Run("no session agent 422", func(t *testing.T) {
		s := newAlignServer(t, "", 0)
		w := do(t, s.Handler(), "POST", "/api/settings/opencode-default-agent", "")
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("code = %d", w.Code)
		}
	})

	t.Run("service unreachable 503", func(t *testing.T) {
		s := newAlignServer(t, "build", http.StatusInternalServerError)
		w := do(t, s.Handler(), "POST", "/api/settings/opencode-default-agent", "")
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("code = %d body = %s", w.Code, w.Body)
		}
	})

	t.Run("jsonc 422 and file untouched", func(t *testing.T) {
		s := mappingServer(t, agentServe(0))
		writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: "build"}})
		path := filepath.Join(s.st.Dir, "opencode.json")
		jsonc := "{ // c\n \"default_agent\": \"kg-orchestrator\"\n}\n"
		if err := os.WriteFile(path, []byte(jsonc), 0o644); err != nil {
			t.Fatal(err)
		}
		w := do(t, s.Handler(), "POST", "/api/settings/opencode-default-agent", "")
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("code = %d body = %s", w.Code, w.Body)
		}
		after, _ := os.ReadFile(path)
		if string(after) != jsonc {
			t.Error("JSONC file must stay byte-identical")
		}
	})

	t.Run("no default_agent key 422", func(t *testing.T) {
		s := mappingServer(t, agentServe(0))
		writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: "build"}})
		writeOpencodeConfig(t, s.st.Dir, "{\"watcher\":{}}\n")
		w := do(t, s.Handler(), "POST", "/api/settings/opencode-default-agent", "")
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("code = %d body = %s", w.Code, w.Body)
		}
	})
}
