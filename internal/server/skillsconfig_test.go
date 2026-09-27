package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchSkillsCatalog(t *testing.T) {
	const url = "http://127.0.0.1:9090/p/tasktracker/skills/"
	t.Run("adds to existing config preserving bytes", func(t *testing.T) {
		in := []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"default_agent\": \"build\"\n}\n")
		out, changed, err := patchSkillsCatalog(in, url)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		for _, want := range []string{
			`"$schema": "https://opencode.ai/config.json"`,
			`"default_agent": "build"`,
			`"skills": ["` + url + `"]`,
		} {
			if !bytes.Contains(out, []byte(want)) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
	t.Run("idempotent no-op when the entry exists", func(t *testing.T) {
		in := []byte("{\n  \"skills\": [\"" + url + "\"],\n  \"model\": \"x/y\"\n}\n")
		out, changed, err := patchSkillsCatalog(in, url)
		if err != nil {
			t.Fatal(err)
		}
		if changed || !bytes.Equal(out, in) {
			t.Errorf("changed=%v out=%s", changed, out)
		}
	})
	t.Run("appends to an existing skills array preserving entries", func(t *testing.T) {
		in := []byte(`{"skills":["~/shared/skills"]}`)
		out, changed, err := patchSkillsCatalog(in, url)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if !bytes.Contains(out, []byte("~/shared/skills")) || !bytes.Contains(out, []byte(url)) {
			t.Errorf("output = %s", out)
		}
	})
	t.Run("replaces a stale lessmess catalog URL", func(t *testing.T) {
		in := []byte(`{"skills":["http://127.0.0.1:9095/skills/"]}`)
		out, changed, err := patchSkillsCatalog(in, url)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if bytes.Contains(out, []byte("9095")) || !bytes.Contains(out, []byte(url)) {
			t.Errorf("stale entry not replaced: %s", out)
		}
	})
	t.Run("stale lessmess URL replaced, foreign entries kept", func(t *testing.T) {
		in := []byte(`{"skills":["~/shared/skills","http://old:1/skills/"]}`)
		out, changed, err := patchSkillsCatalog(in, url)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		for _, want := range []string{"~/shared/skills", url} {
			if !bytes.Contains(out, []byte(want)) {
				t.Errorf("output missing %q: %s", want, out)
			}
		}
		if bytes.Contains(out, []byte("old:1")) {
			t.Errorf("stale lessmess entry survived: %s", out)
		}
	})
	t.Run("seeds an empty object", func(t *testing.T) {
		out, changed, err := patchSkillsCatalog([]byte("{}"), url)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if !bytes.Contains(out, []byte(url)) {
			t.Errorf("output = %s", out)
		}
	})
	t.Run("refuses JSONC", func(t *testing.T) {
		if _, _, err := patchSkillsCatalog([]byte("{\n // comment\n \"a\": 1\n}"), url); err == nil {
			t.Error("expected an error for JSONC input")
		}
	})
}

func TestEnsureSkillsCatalog(t *testing.T) {
	t.Run("creates the file when absent", func(t *testing.T) {
		s := mappingServer(t, nil)
		s.PublicBase = "127.0.0.1:9090"
		s.EnsureSkillsCatalog()
		b, err := os.ReadFile(filepath.Join(s.st.Dir, "opencode.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "http://127.0.0.1:9090/skills/") {
			t.Errorf("opencode.json = %s", b)
		}
	})
	t.Run("patches and is idempotent", func(t *testing.T) {
		s := mappingServer(t, nil)
		s.PublicBase = "127.0.0.1:9090"
		path := filepath.Join(s.st.Dir, "opencode.json")
		if err := os.WriteFile(path, []byte("{\n  \"model\": \"a/b\"\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		s.EnsureSkillsCatalog()
		first, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(first), `"skills"`) {
			t.Errorf("opencode.json = %s", first)
		}
		s.EnsureSkillsCatalog()
		second, _ := os.ReadFile(path)
		if !bytes.Equal(first, second) {
			t.Errorf("second call rewrote the file:\nfirst=%s\nsecond=%s", first, second)
		}
	})
	t.Run("refuses to rewrite JSONC", func(t *testing.T) {
		s := mappingServer(t, nil)
		path := filepath.Join(s.st.Dir, "opencode.json")
		jsonc := "{\n // mine\n \"model\": \"a/b\"\n}\n"
		if err := os.WriteFile(path, []byte(jsonc), 0o644); err != nil {
			t.Fatal(err)
		}
		s.EnsureSkillsCatalog()
		b, _ := os.ReadFile(path)
		if string(b) != jsonc {
			t.Errorf("JSONC file was rewritten:\n%s", b)
		}
	})
}
