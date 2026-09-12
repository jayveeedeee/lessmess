package server

import (
	"encoding/json"
	"testing"

	"tasktracker/internal/docs"
)

func TestValidateEndpointIncludesDocsFindings(t *testing.T) {
	s, _ := docsServer(t) // docs enabled; repo is unseeded → warnings expected
	w := do(t, s.Handler(), "GET", "/api/validate", "")
	if w.Code != 200 {
		t.Fatalf("validate: %d", w.Code)
	}
	var resp struct {
		Violations []json.RawMessage `json:"violations"`
		Docs       []docs.Finding    `json:"docs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Docs) == 0 {
		t.Fatal("unseeded docs-enabled repo must produce docs findings")
	}
	for _, f := range resp.Docs {
		if f.Severity != docs.SeverityWarning {
			t.Errorf("unseeded repo must warn, not error: %v", f)
		}
	}
	// The banner JS consumes lowercase keys; lock the wire format.
	var wire struct {
		Docs []map[string]any `json:"docs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	for _, f := range wire.Docs {
		for _, key := range []string{"severity", "file", "msg"} {
			if _, ok := f[key]; !ok {
				t.Errorf("finding missing wire key %q: %v", key, f)
			}
		}
	}
}

func TestValidateEndpointDocsEmptyWhenDisabled(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	t.Cleanup(s.Close)
	w := do(t, s.Handler(), "GET", "/api/validate", "")
	var resp struct {
		Docs []docs.Finding `json:"docs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Docs) != 0 {
		t.Errorf("docs disabled must yield no findings: %v", resp.Docs)
	}
}
