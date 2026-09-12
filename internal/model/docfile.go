package model

import (
	"strings"
)

// Markers delimiting the machine-maintained "auto" section of a repo docs
// file (STRUCTURE.md or AGENTS.md). Everything outside the markers is
// human/curated content and is preserved byte-for-byte.
const (
	DocMarkerBegin = "<!-- tasktracker:begin -->"
	DocMarkerEnd   = "<!-- tasktracker:end -->"
)

// DocFile is a repo docs file split around its auto section.
type DocFile struct {
	Prefix  string // content before the begin marker
	Auto    string // machine-maintained content between the markers
	Suffix  string // content after the end marker
	HasAuto bool   // false when the file has no marker section
}

// ParseDocFile splits data around the auto-section markers. A file without
// markers parses as purely human content. Multiple marker pairs, an end
// marker without a begin marker, an end marker before the begin marker, or
// an unterminated section are errors.
func ParseDocFile(name string, data []byte) (*DocFile, error) {
	s := string(data)
	if strings.Count(s, DocMarkerBegin) > 1 || strings.Count(s, DocMarkerEnd) > 1 {
		return nil, parseErr(name, "multiple tasktracker marker pairs")
	}
	i := strings.Index(s, DocMarkerBegin)
	j := strings.Index(s, DocMarkerEnd)
	switch {
	case i < 0 && j < 0:
		return &DocFile{Prefix: s}, nil
	case i < 0:
		return nil, parseErr(name, "end marker without begin marker")
	case j < 0:
		return nil, parseErr(name, "unterminated auto section: end marker missing")
	case j < i:
		return nil, parseErr(name, "end marker before begin marker")
	}
	return &DocFile{
		Prefix:  s[:i],
		Auto:    s[i+len(DocMarkerBegin) : j],
		Suffix:  s[j+len(DocMarkerEnd):],
		HasAuto: true,
	}, nil
}

// Render reassembles the file. Rendering a freshly parsed DocFile yields the
// original bytes exactly.
func (d *DocFile) Render() []byte {
	if !d.HasAuto {
		return []byte(d.Prefix)
	}
	var b strings.Builder
	b.WriteString(d.Prefix)
	b.WriteString(DocMarkerBegin)
	b.WriteString(d.Auto)
	b.WriteString(DocMarkerEnd)
	b.WriteString(d.Suffix)
	return []byte(b.String())
}

// MergeDoc returns existing with its auto section replaced by newAuto,
// preserving all bytes outside the markers. A missing file is created; a
// file without markers gains the auto section appended after its human
// content. Corrupt marker structure in existing is an error (the caller
// must refuse to write, per the project's no-clobber discipline).
func MergeDoc(name string, existing, newAuto []byte) ([]byte, error) {
	auto := strings.TrimRight(string(newAuto), "\n")
	if len(existing) == 0 {
		return []byte(DocMarkerBegin + "\n" + auto + "\n" + DocMarkerEnd + "\n"), nil
	}
	d, err := ParseDocFile(name, existing)
	if err != nil {
		return nil, err
	}
	if !d.HasAuto {
		prefix := strings.TrimRight(d.Prefix, "\n")
		return []byte(prefix + "\n\n" + DocMarkerBegin + "\n" + auto + "\n" + DocMarkerEnd + "\n"), nil
	}
	d.Auto = "\n" + auto + "\n"
	return d.Render(), nil
}

// DocMetaPrefix starts the freshness-metadata line inside an auto section.
const DocMetaPrefix = "<!-- tasktracker-meta:"

// DocMeta is the freshness provenance stamped into an auto section: when it
// was refreshed, by which change (or "seed"/"manual"), and the covered-tree
// hash the content was generated from (STRUCTURE.md only).
type DocMeta struct {
	Refreshed string // YYYY-MM-DD
	Source    string // change ID, "seed", or "manual"
	TreeHash  string // covered-tree hash; empty for AGENTS.md
}

// String renders the metadata line.
func (m *DocMeta) String() string {
	fields := "refreshed=" + m.Refreshed + " source=" + m.Source
	if m.TreeHash != "" {
		fields += " tree=" + m.TreeHash
	}
	return DocMetaPrefix + " " + fields + " -->"
}

// ParseDocMeta extracts the freshness metadata from an auto section. It
// returns nil without error when the section carries no metadata line.
// Unknown keys are ignored for forward compatibility.
func ParseDocMeta(name, auto string) (*DocMeta, error) {
	for _, line := range strings.Split(auto, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, DocMetaPrefix) {
			continue
		}
		inner := strings.TrimPrefix(line, DocMetaPrefix)
		if !strings.HasSuffix(strings.TrimSpace(inner), "-->") {
			return nil, parseErr(name, "tasktracker-meta line not closed with -->")
		}
		inner = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(inner), "-->"))
		var m DocMeta
		for _, f := range strings.Fields(inner) {
			k, v, ok := strings.Cut(f, "=")
			if !ok {
				return nil, parseErr(name, "malformed tasktracker-meta field %q", f)
			}
			switch k {
			case "refreshed":
				m.Refreshed = v
			case "source":
				m.Source = v
			case "tree":
				m.TreeHash = v
			default:
				// Ignore unknown keys for forward compatibility.
			}
		}
		if m.Refreshed == "" || m.Source == "" {
			return nil, parseErr(name, "tasktracker-meta requires refreshed and source")
		}
		return &m, nil
	}
	return nil, nil
}
