package terminal

import (
	"strings"
	"testing"
	"time"
)

// readUntil accumulates PTY output until want appears or the deadline passes.
func readUntil(t *testing.T, p *PTY, want string, timeout time.Duration) string {
	t.Helper()
	type chunk struct {
		b   []byte
		err error
	}
	ch := make(chan chunk)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.Read(buf)
			ch <- chunk{append([]byte(nil), buf[:n]...), err}
			if err != nil {
				return
			}
		}
	}()
	var got strings.Builder
	deadline := time.After(timeout)
	for !strings.Contains(got.String(), want) {
		select {
		case c := <-ch:
			if c.err != nil {
				t.Fatalf("read: %v (got %q so far)", c.err, got.String())
			}
			got.Write(c.b)
		case <-deadline:
			t.Fatalf("timed out waiting for %q, got %q", want, got.String())
		}
	}
	return got.String()
}

func TestSpawnWithEnvPassesEnv(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.CloseAll)
	p, err := m.SpawnWithEnv("sh", []string{"-c", `echo "VAR=$TUI_TEST_VAR"; cat`}, t.TempDir(), 80, 24,
		[]string{"TUI_TEST_VAR=hello-pty"})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	readUntil(t, p, "VAR=hello-pty", 5*time.Second)
}

func TestSpawnNilEnvInherits(t *testing.T) {
	t.Setenv("TUI_INHERIT_VAR", "from-parent")
	m := NewManager()
	t.Cleanup(m.CloseAll)
	p, err := m.Spawn("sh", []string{"-c", `echo "VAR=$TUI_INHERIT_VAR"; cat`}, t.TempDir(), 80, 24)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	readUntil(t, p, "VAR=from-parent", 5*time.Second)
}
