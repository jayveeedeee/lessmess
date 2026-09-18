package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func uint16Or(q string, def uint16) uint16 {
	if n, err := strconv.ParseUint(q, 10, 16); err == nil && n > 0 {
		return uint16(n)
	}
	return def
}

// terminalDir picks the working directory for a session's terminal: the
// bound change's worktree when the session is change-bound and that change
// has one, else the main tree.
func (s *Server) terminalDir(sessionID string) string {
	if s.mapErr == nil {
		if changeID, ok := s.sessions.changeOf(sessionID); ok {
			return s.changeSessionDir(changeID)
		}
	}
	return s.st.Dir
}

// terminalWS upgrades GET /terminal/ws?session={id} to a WebSocket bridged
// to a fresh in-process PTY running the opencode TUI on that session. The
// PTY is killed when the socket closes; the session persists in the
// opencode service.
func (s *Server) terminalWS(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if !strings.HasPrefix(sessionID, "ses_") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ses_ session id required"})
		return
	}
	spawn := s.SpawnCommand
	name, args := spawn(sessionID)
	cols, rows := uint16Or(r.URL.Query().Get("cols"), 120), uint16Or(r.URL.Query().Get("rows"), 30)
	// Embedded sessions get a lessmess-managed CLI config (no tab strip, no
	// sidebar); any failure falls back to the inherited environment.
	env := []string(nil)
	if xdg, err := ensureTUIConfig(s.st.Dir); err != nil {
		slog.Warn("tui config: spawning with default environment", "err", err)
	} else {
		env = xdgEnv(xdg)
	}
	// The PTY runs where the session lives: a change-bound session's
	// worktree when it has one, else the main tree.
	cwd := s.terminalDir(sessionID)
	t, err := s.term.SpawnWithEnv(name, args, cwd, cols, rows, env)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer t.Close()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{r.Host}, // same-origin only
	})
	if err != nil {
		return // Accept already wrote the error response
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")

	// PTY → WS
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := t.Read(buf)
			if n > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = conn.Write(ctx, websocket.MessageBinary, buf[:n])
				cancel()
			}
			if err != nil {
				conn.Close(websocket.StatusNormalClosure, "process exited")
				return
			}
		}
	}()

	// WS → PTY (JSON control frames vs raw input)
	for {
		typ, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		if typ == websocket.MessageText {
			var m resizeMsg
			if json.Unmarshal(data, &m) == nil && m.Type == "resize" && m.Cols > 0 && m.Rows > 0 {
				_ = t.Resize(m.Cols, m.Rows)
				continue
			}
		}
		if _, err := t.Write(data); err != nil {
			return
		}
	}
}
