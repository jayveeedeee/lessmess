package model

import (
	"os"
	"strings"
	"testing"
)

func TestParseDocFileRoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/doc_structure.md")
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDocFile("doc_structure.md", data)
	if err != nil {
		t.Fatal(err)
	}
	if !d.HasAuto {
		t.Fatal("expected auto section")
	}
	if d.Prefix != "" || d.Suffix != "\n" {
		t.Errorf("unexpected prefix/suffix: %q / %q", d.Prefix, d.Suffix)
	}
	if got := d.Render(); string(got) != string(data) {
		t.Errorf("render not byte-identical:\n--- got ---\n%s\n--- want ---\n%s", got, data)
	}
}

func TestParseDocFileHumanOnly(t *testing.T) {
	data, err := os.ReadFile("testdata/doc_agents_human.md")
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDocFile("doc_agents_human.md", data)
	if err != nil {
		t.Fatal(err)
	}
	if d.HasAuto {
		t.Fatal("expected no auto section")
	}
	if got := d.Render(); string(got) != string(data) {
		t.Error("render not byte-identical for human-only file")
	}
}

func TestParseDocFileErrors(t *testing.T) {
	cases := map[string]string{
		"end without begin": "text\n" + DocMarkerEnd + "\n",
		"unterminated":      DocMarkerBegin + "\nauto\n",
		"end before begin":  DocMarkerEnd + " x " + DocMarkerBegin,
		"multiple begins":   DocMarkerBegin + "\n" + DocMarkerBegin + "\n" + DocMarkerEnd,
		"multiple ends":     DocMarkerBegin + "\n" + DocMarkerEnd + "\n" + DocMarkerEnd,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseDocFile("x.md", []byte(in)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMergeDocCreate(t *testing.T) {
	got, err := MergeDoc("STRUCTURE.md", nil, []byte("# Structure\n\nentries\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := DocMarkerBegin + "\n# Structure\n\nentries\n" + DocMarkerEnd + "\n"
	if string(got) != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestMergeDocAppendToHumanFile(t *testing.T) {
	human, err := os.ReadFile("testdata/doc_agents_human.md")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("testdata/doc_agents_merged.md")
	if err != nil {
		t.Fatal(err)
	}
	auto := "## Recent changes\n\n- 2026-09-12-7 — seeded\n"
	got, err := MergeDoc("AGENTS.md", human, []byte(auto))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(golden) {
		t.Errorf("merge mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, golden)
	}
	if !strings.HasPrefix(string(got), strings.TrimRight(string(human), "\n")) {
		t.Error("human content not preserved at head of merged file")
	}
}

func TestMergeDocReplacePreservesHumanBytes(t *testing.T) {
	orig, err := os.ReadFile("testdata/doc_structure.md")
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDocFile("doc_structure.md", orig)
	if err != nil {
		t.Fatal(err)
	}
	got, err := MergeDoc("doc_structure.md", orig, []byte("# Structure\n\nnew body\n"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseDocFile("doc_structure.md", got)
	if err != nil {
		t.Fatal(err)
	}
	if g.Prefix != d.Prefix || g.Suffix != d.Suffix {
		t.Errorf("human bytes changed: prefix %q→%q suffix %q→%q", d.Prefix, g.Prefix, d.Suffix, g.Suffix)
	}
	if !strings.Contains(g.Auto, "new body") {
		t.Errorf("auto section not replaced: %q", g.Auto)
	}
	if strings.Contains(g.Auto, "main.go") {
		t.Error("old auto content survived replacement")
	}
}

func TestMergeDocRefusesCorruptMarkers(t *testing.T) {
	corrupt := []byte("intro\n" + DocMarkerEnd + "\n")
	if _, err := MergeDoc("AGENTS.md", corrupt, []byte("x")); err == nil {
		t.Fatal("expected error for corrupt markers")
	}
}

func TestDocMetaRoundTrip(t *testing.T) {
	m := &DocMeta{Refreshed: "2026-09-12", Source: "2026-09-12-7", TreeHash: "9f2c1a"}
	auto := "# Structure\n\n" + m.String() + "\n\nbody\n"
	got, err := ParseDocMeta("STRUCTURE.md", auto)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != *m {
		t.Errorf("got %+v want %+v", got, m)
	}

	noTree := &DocMeta{Refreshed: "2026-09-12", Source: "manual"}
	got, err = ParseDocMeta("AGENTS.md", noTree.String())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != *noTree {
		t.Errorf("got %+v want %+v", got, noTree)
	}
}

func TestParseDocMetaFromFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/doc_structure.md")
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDocFile("doc_structure.md", data)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseDocMeta("doc_structure.md", d.Auto)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.Refreshed != "2026-09-12" || m.Source != "seed" || m.TreeHash != "9f2c1a" {
		t.Errorf("unexpected meta: %+v", m)
	}
}

func TestParseDocMetaAbsentAndBad(t *testing.T) {
	m, err := ParseDocMeta("x.md", "# just content\nno meta here\n")
	if err != nil || m != nil {
		t.Errorf("absent meta: got %v, %v", m, err)
	}
	bad := []string{
		DocMetaPrefix + " refreshed=2026-09-12\n",   // unterminated
		DocMetaPrefix + " refreshed -->",            // malformed field
		DocMetaPrefix + " refreshed=2026-09-12 -->", // missing source
		DocMetaPrefix + " source=seed -->",          // missing refreshed
	}
	for _, in := range bad {
		if _, err := ParseDocMeta("x.md", in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
	// Unknown keys are tolerated.
	m, err = ParseDocMeta("x.md", DocMetaPrefix+" refreshed=2026-09-12 source=seed future=x -->")
	if err != nil || m == nil || m.Source != "seed" {
		t.Errorf("unknown key: got %v, %v", m, err)
	}
}
