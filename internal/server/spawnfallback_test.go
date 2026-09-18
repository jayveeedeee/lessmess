package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The ladder preserves whichever configured value the service still accepts:
// a rejected model must not cost the agent, and vice versa.

func TestSpawnLadderKeepsAgentWhenModelRejected(t *testing.T) {
	cap := &ocCapture{rejectModel: true}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{
		Agent: "build",
		Model: "prov/gone-m",
	}})

	if w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`); w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	if len(cap.creates) != 2 {
		t.Fatalf("creates = %d, want 2 (agent+model, then agent-only)", len(cap.creates))
	}
	if _, ok := cap.creates[0]["model"]; !ok {
		t.Errorf("first attempt must carry the model: %v", cap.creates[0])
	}
	if cap.creates[1]["agent"] != "build" {
		t.Errorf("fallback agent = %v, want build (a bad model must not cost the agent)", cap.creates[1]["agent"])
	}
	if _, ok := cap.creates[1]["model"]; ok {
		t.Errorf("fallback must drop the rejected model: %v", cap.creates[1])
	}
}

func TestSpawnLadderKeepsModelWhenAgentRejected(t *testing.T) {
	cap := &ocCapture{rejectAgent: true}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{
		Agent: "ghost",
		Model: "prov/kept-m",
	}})

	if w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`); w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	// agent+model, agent-only, model-only (success).
	if len(cap.creates) != 3 {
		t.Fatalf("creates = %d, want 3", len(cap.creates))
	}
	if _, ok := cap.creates[2]["agent"]; ok {
		t.Errorf("model-only retry must drop the rejected agent: %v", cap.creates[2])
	}
	m, _ := cap.creates[2]["model"].(map[string]any)
	if m["providerID"] != "prov" || m["id"] != "kept-m" {
		t.Errorf("model = %v, want prov/kept-m (a bad agent must not cost the model)", cap.creates[2]["model"])
	}
}

func TestSpawnLadderAgentOnlyConfigured(t *testing.T) {
	cap := &ocCapture{rejectAgent: true}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: "ghost"}})

	if w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`); w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	// agent-only, then plain — no model-only duplication.
	if len(cap.creates) != 2 {
		t.Fatalf("creates = %d, want 2", len(cap.creates))
	}
	if cap.creates[0]["agent"] != "ghost" {
		t.Errorf("first attempt agent = %v, want ghost", cap.creates[0]["agent"])
	}
	if _, ok := cap.creates[1]["agent"]; ok {
		t.Errorf("plain retry must drop the agent: %v", cap.creates[1])
	}
}

func TestSpawnLadderStopsOnNon400(t *testing.T) {
	cap := &ocCapture{agentStatus: http.StatusInternalServerError}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{
		Agent: "build",
		Model: "prov/m",
	}})

	_, err := spawnSessionWithModel(context.Background(), s.oc, s.st.Dir, s.st.Dir, "t", "")
	if err == nil {
		t.Fatal("expected the non-400 error to propagate")
	}
	if !strings.Contains(err.Error(), "opencode API 500") {
		t.Errorf("err = %v, want the 500 API error", err)
	}
	if len(cap.creates) != 1 {
		t.Fatalf("creates = %d, want 1 (ladder stops on non-400)", len(cap.creates))
	}
}

func TestSpawnLadderPlainFailureReturnsError(t *testing.T) {
	s := mappingServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"message":"boom"}`))
	}))
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: "build", Model: "prov/m"}})

	if _, err := spawnSessionWithModel(context.Background(), s.oc, s.st.Dir, s.st.Dir, "t", ""); err == nil {
		t.Fatal("expected an error when every attempt fails")
	}
}

// --- fallback record and banner (SPF-01) ---

func TestSpawnFallbackRecordLifecycle(t *testing.T) {
	cap := &ocCapture{rejectModel: true}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{
		Agent: "build",
		Model: "prov/gone-m",
	}})

	if w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`); w.Code != 201 {
		t.Fatalf("fallback spawn: %d %s", w.Code, w.Body)
	}
	rec := readSpawnFallback(s.st.Dir)
	if rec == nil {
		t.Fatal("fallback spawn must persist a record")
	}
	if rec.Outcome != "agent-only" {
		t.Errorf("outcome = %q, want agent-only", rec.Outcome)
	}
	if rec.AttemptedAgent != "build" || rec.AttemptedModel != "prov/gone-m" {
		t.Errorf("attempted = %q/%q, want build / prov/gone-m", rec.AttemptedAgent, rec.AttemptedModel)
	}
	if !strings.Contains(rec.ServiceError, "unknown model") {
		t.Errorf("serviceError = %q, want the model rejection", rec.ServiceError)
	}
	if rec.Time == "" {
		t.Error("record must carry a timestamp")
	}

	// A clean spawn (agent-only configured, accepted on step 1) clears it.
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: "build"}})
	if w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t2"}`); w.Code != 201 {
		t.Fatalf("clean spawn: %d %s", w.Code, w.Body)
	}
	if rec := readSpawnFallback(s.st.Dir); rec != nil {
		t.Errorf("clean spawn must clear the record, got %+v", rec)
	}
}

func TestSpawnFallbackRecordFailOpen(t *testing.T) {
	s := mappingServer(t, (&ocCapture{}).handler())
	path := spawnFallbackPath(s.st.Dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json {"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := readSpawnFallback(s.st.Dir); rec != nil {
		t.Errorf("malformed record must read as nil, got %+v", rec)
	}
	w := htmlGet(t, s.Handler(), "/", false)
	if w.Code != 200 {
		t.Fatalf("index: %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "spawn-fallback-banner") {
		t.Error("malformed record must not render a banner")
	}
}

func TestIndexShowsFallbackBanner(t *testing.T) {
	cap := &ocCapture{rejectModel: true}
	s := mappingServer(t, cap.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{
		Agent: "build",
		Model: "prov/gone-m",
	}})
	if w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"t"}`); w.Code != 201 {
		t.Fatalf("fallback spawn: %d %s", w.Code, w.Body)
	}

	w := htmlGet(t, s.Handler(), "/", false)
	if w.Code != 200 {
		t.Fatalf("index: %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"spawn-fallback-banner", "Spawn fallback", "build", "Review settings"} {
		if !strings.Contains(body, want) {
			t.Errorf("index HTML missing %q", want)
		}
	}

	// Without a record there is no banner.
	s2 := mappingServer(t, (&ocCapture{}).handler())
	w = htmlGet(t, s2.Handler(), "/", false)
	if w.Code != 200 {
		t.Fatalf("clean index: %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "spawn-fallback-banner") {
		t.Error("banner must be absent without a fallback record")
	}
}
