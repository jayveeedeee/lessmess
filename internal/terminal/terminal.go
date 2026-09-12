// Package terminal manages in-process PTYs running interactive TUIs
// (opencode2 --session <id>) for bridging to browser WebSockets.
package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// PTY is one running pseudo-terminal process.
type PTY struct {
	ID  string
	cmd *exec.Cmd
	f   *os.File
}

// Write sends input to the process.
func (p *PTY) Write(b []byte) (int, error) { return p.f.Write(b) }

// Read reads process output.
func (p *PTY) Read(b []byte) (int, error) { return p.f.Read(b) }

// Resize sets the PTY window size.
func (p *PTY) Resize(cols, rows uint16) error {
	return pty.Setsize(p.f, &pty.Winsize{Cols: cols, Rows: rows})
}

// Close terminates the process and releases the PTY.
func (p *PTY) Close() error {
	_ = p.f.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = p.cmd.Wait()
	return nil
}

// Manager tracks running PTYs for lookup and shutdown cleanup.
type Manager struct {
	mu   sync.Mutex
	ptys map[string]*PTY
	next int
}

// NewManager builds an empty Manager.
func NewManager() *Manager { return &Manager{ptys: map[string]*PTY{}} }

// Spawn starts name+args in a new PTY at cwd with the given window size,
// inheriting the server process environment.
func (m *Manager) Spawn(name string, args []string, cwd string, cols, rows uint16) (*PTY, error) {
	return m.SpawnWithEnv(name, args, cwd, cols, rows, nil)
}

// SpawnWithEnv is Spawn with an explicit process environment; a nil env
// inherits the server process environment.
func (m *Manager) SpawnWithEnv(name string, args []string, cwd string, cols, rows uint16, env []string) (*PTY, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	if env != nil {
		cmd.Env = env
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}
	m.mu.Lock()
	m.next++
	p := &PTY{ID: fmt.Sprintf("pty-%d", m.next), cmd: cmd, f: f}
	m.ptys[p.ID] = p
	m.mu.Unlock()
	return p, nil
}

// CloseAll terminates every running PTY (server shutdown).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.ptys {
		_ = p.Close()
		delete(m.ptys, id)
	}
}
