package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSkillCatalogIndex(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "GET", "/skills/index.json", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp struct {
		Skills []struct {
			Name    string   `json:"name"`
			Version string   `json:"version"`
			Files   []string `json:"files"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	wantNames := []string{"lessmess-scaffold", "lessmess-task", "lessmess-handoff", "lessmess-closeout"}
	if len(resp.Skills) != len(wantNames) {
		t.Fatalf("skills = %d, want %d", len(resp.Skills), len(wantNames))
	}
	for i, want := range wantNames {
		got := resp.Skills[i]
		if got.Name != want {
			t.Errorf("skill[%d] = %q, want %q", i, got.Name, want)
		}
		if got.Version == "" {
			t.Errorf("skill %s: empty version", want)
		}
		if len(got.Files) != 1 || got.Files[0] != want+".md" {
			t.Errorf("skill %s: files = %v", want, got.Files)
		}
	}
	// The version is a stable content hash: identical requests agree.
	w2 := do(t, s.Handler(), "GET", "/skills/index.json", "")
	var resp2 struct {
		Skills []struct {
			Version string `json:"version"`
		} `json:"skills"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp.Skills[0].Version != resp2.Skills[0].Version {
		t.Errorf("version unstable: %q vs %q", resp.Skills[0].Version, resp2.Skills[0].Version)
	}
}

func TestSkillFileServed(t *testing.T) {
	s := mappingServer(t, nil)
	s.PublicBase = "127.0.0.1:9090"
	w := do(t, s.Handler(), "GET", "/skills/lessmess-scaffold/lessmess-scaffold.md", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{
		"---\nname: Lessmess Scaffold\ndescription: ",
		"curl -s -X POST http://127.0.0.1:9090/changes/scaffold",
		`"session":"<your session id>"`,
		"lessmess-task",
		"api/validate",
		"explicitly approves",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scaffold skill missing %q", want)
		}
	}
	if strings.Contains(body, "{{apiBase}}") {
		t.Error("scaffold skill has an unsubstituted apiBase placeholder")
	}
	// Free-chat path: the body welcomes unbound sessions instead of
	// guarding them away.
	if !strings.Contains(body, "any unbound session") {
		t.Error("scaffold skill must welcome unbound (free chat) sessions")
	}
}

func TestSkillFileBodiesCarryGates(t *testing.T) {
	s := mappingServer(t, nil)
	cases := map[string]string{
		"lessmess-task":     "Never edit .lessmess/workflow/",
		"lessmess-handoff":  "explicit approval",
		"lessmess-closeout": "Only the user moves tasks to Done",
	}
	for name, want := range cases {
		w := do(t, s.Handler(), "GET", "/skills/"+name+"/"+name+".md", "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: code = %d", name, w.Code)
		}
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s: body missing %q", name, want)
		}
	}
	// Handoff is change-bound-only and redirects unbound sessions to
	// scaffold.
	w := do(t, s.Handler(), "GET", "/skills/lessmess-handoff/lessmess-handoff.md", "")
	if !strings.Contains(w.Body.String(), "lessmess-scaffold") {
		t.Error("handoff skill must redirect unbound sessions to scaffold")
	}
}

func TestSkillFileRejectsUnknownAndMismatched(t *testing.T) {
	s := mappingServer(t, nil)
	if w := do(t, s.Handler(), "GET", "/skills/lessmess-nope/lessmess-nope.md", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown skill code = %d", w.Code)
	}
	if w := do(t, s.Handler(), "GET", "/skills/lessmess-task/other.md", ""); w.Code != http.StatusNotFound {
		t.Errorf("mismatched file code = %d", w.Code)
	}
}
