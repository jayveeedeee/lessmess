package model

import (
	"regexp"
	"strings"
)

// Table is a parsed markdown table within a document's lines.
type Table struct {
	Start int      // line index of the header row
	End   int      // line index one past the last data row
	Cols  []string // header cells
	Rows  [][]string
}

var (
	sepCell = regexp.MustCompile(`^:?-+:?$`)
	linkRe  = regexp.MustCompile(`^\[([^\]]*)\]\(([^)]*)\)$`)
)

func isTableLine(s string) bool {
	t := strings.TrimSpace(s)
	return len(t) >= 2 && strings.HasPrefix(t, "|") && strings.HasSuffix(t, "|")
}

func splitRow(line string) []string {
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparator(line string) bool {
	if !isTableLine(line) {
		return false
	}
	for _, c := range splitRow(line) {
		if !sepCell.MatchString(c) {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FindTable locates the table whose header matches cols exactly.
func FindTable(lines []string, cols []string, file string) (*Table, error) {
	for i := 0; i+1 < len(lines); i++ {
		if !isTableLine(lines[i]) || !isSeparator(lines[i+1]) {
			continue
		}
		if !equalStrings(splitRow(lines[i]), cols) {
			continue
		}
		t := &Table{Start: i, Cols: cols}
		j := i + 2
		for j < len(lines) && isTableLine(lines[j]) {
			row := splitRow(lines[j])
			if len(row) != len(cols) {
				return nil, parseErr(file, "table row %d has %d cells, want %d (literal | in cell text?)", j+1, len(row), len(cols))
			}
			t.Rows = append(t.Rows, row)
			j++
		}
		t.End = j
		return t, nil
	}
	return nil, parseErr(file, "table with columns %q not found", strings.Join(cols, " | "))
}

// RenderTable renders a markdown table with the canonical schema style.
func RenderTable(cols []string, rows [][]string) []string {
	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, "| "+strings.Join(cols, " | ")+" |")
	sep := make([]string, len(cols))
	for i := range sep {
		sep[i] = "---"
	}
	lines = append(lines, "| "+strings.Join(sep, " | ")+" |")
	for _, r := range rows {
		lines = append(lines, "| "+strings.Join(r, " | ")+" |")
	}
	return lines
}

// Splice replaces lines[start:end] with repl, returning a new slice.
func Splice(lines []string, start, end int, repl []string) []string {
	out := make([]string, 0, len(lines)-(end-start)+len(repl))
	out = append(out, lines[:start]...)
	out = append(out, repl...)
	out = append(out, lines[end:]...)
	return out
}

// ParseLink extracts text and href from a [text](href) cell.
func ParseLink(cell string) (text, href string, ok bool) {
	m := linkRe.FindStringSubmatch(strings.TrimSpace(cell))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// FormatLink renders a [text](href) cell.
func FormatLink(text, href string) string { return "[" + text + "](" + href + ")" }
