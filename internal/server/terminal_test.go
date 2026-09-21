package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"lessmess/internal/store"
)

func terminalTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SpawnCommand = func(sessionID string) (string, []string) { return "cat", nil }
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		srv.Close()
		s.Close()
	})
	return srv, s
}

func wsURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

func TestTerminalWSEcho(t *testing.T) {
	srv, _ := terminalTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, wsURL(srv, "/terminal/ws?session=ses_test"), nil)
	if err != nil {
		t.Fatalf("dial: %v (resp %v)", err, resp)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	msg := []byte("hello pty")
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	// cat echoes it back (possibly wrapped in terminal echo of the input).
	deadline := time.Now().Add(5 * time.Second)
	var got strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(got.String(), "hello pty") {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v (got %q so far)", err, got.String())
		}
		got.Write(data)
	}
	if !strings.Contains(got.String(), "hello pty") {
		t.Fatalf("no echo received, got %q", got.String())
	}
}

func TestTerminalWSResize(t *testing.T) {
	srv, _ := terminalTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL(srv, "/terminal/ws?session=ses_test"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":80,"rows":24}`)); err != nil {
		t.Fatalf("resize write: %v", err)
	}
	// Follow with input to prove the bridge is still functional after a control frame.
	if err := conn.Write(ctx, websocket.MessageText, []byte("after-resize")); err != nil {
		t.Fatalf("write: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var got strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(got.String(), "after-resize") {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		got.Write(data)
	}
}

func TestTerminalWSBadSession(t *testing.T) {
	srv, _ := terminalTestServer(t)
	resp, err := http.Get(srv.URL + "/terminal/ws?session=nope")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// wsReadUntil reads WS frames until want appears in the accumulated output.
func wsReadUntil(t *testing.T, ctx context.Context, conn *websocket.Conn, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var got strings.Builder
	for time.Now().Before(deadline) && !strings.Contains(got.String(), want) {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v (got %q so far)", err, got.String())
		}
		got.Write(data)
	}
	if !strings.Contains(got.String(), want) {
		t.Fatalf("no %q received, got %q", want, got.String())
	}
	return got.String()
}

// envEchoSpawn makes the spawned process report its XDG_CONFIG_HOME.
func envEchoSpawn(sessionID string) (string, []string) {
	return "sh", []string{"-c", `echo "XDG=$XDG_CONFIG_HOME"; cat`}
}

func TestTerminalWSInjectsTUIConfigEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no user cli.json
	srv, s := terminalTestServer(t)
	s.SpawnCommand = envEchoSpawn
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL(srv, "/terminal/ws?session=ses_test"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	want := "XDG=" + filepath.Join(s.st.Dir, store.StateDirName, "xdg")
	wsReadUntil(t, ctx, conn, want)
	// The generated config must exist where the child was pointed.
	if _, err := os.Stat(filepath.Join(s.st.Dir, store.StateDirName, "xdg", "opencode", "cli.json")); err != nil {
		t.Fatalf("generated cli.json missing: %v", err)
	}
}

func TestTerminalWSConfigFailureFallsBack(t *testing.T) {
	inherited := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", inherited)
	srv, s := terminalTestServer(t)
	s.SpawnCommand = envEchoSpawn
	// Break config generation: .lessmess/xdg exists as a file, so
	// MkdirAll for xdg/opencode fails.
	if err := os.WriteFile(filepath.Join(s.st.Dir, store.StateDirName, "xdg"), []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL(srv, "/terminal/ws?session=ses_test"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// The terminal still opens, with the inherited (unmodified) environment.
	wsReadUntil(t, ctx, conn, "XDG="+inherited)
	if err := conn.Write(ctx, websocket.MessageText, []byte("still-alive")); err != nil {
		t.Fatalf("write: %v", err)
	}
	wsReadUntil(t, ctx, conn, "still-alive")
}

func TestTerminalWSOriginRejected(t *testing.T) {
	srv, _ := terminalTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"https://evil.example.com"}},
	}
	_, resp, err := websocket.Dial(ctx, wsURL(srv, "/terminal/ws?session=ses_test"), opts)
	if err == nil {
		t.Fatal("expected origin rejection")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}
