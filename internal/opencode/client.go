// Package opencode is a minimal client for the opencode background
// service HTTP API, covering the endpoints lessmess needs.
package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Client talks to one opencode service over HTTP Basic auth.
type Client struct {
	base     string
	password string
	hc       *http.Client
}

// New builds a client for base (e.g. http://127.0.0.1:49374) using HTTP
// Basic auth with user "opencode" and the given password.
func New(base, password string) *Client {
	return &Client{
		base:     strings.TrimRight(base, "/"),
		password: password,
		hc:       &http.Client{Timeout: 30 * time.Second},
	}
}

// BaseURL returns the service base URL.
func (c *Client) BaseURL() string { return c.base }

var serviceURLRe = regexp.MustCompile(`https?://[^\s"']+`)

// parseServiceURL extracts the first URL from `opencode2 service status` output.
func parseServiceURL(out string) string {
	return serviceURLRe.FindString(out)
}

// Discover runs `opencode2 service status` and returns the service base URL.
func Discover(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "opencode2", "service", "status")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("opencode2 service status: %w", err)
	}
	u := parseServiceURL(string(out))
	if u == "" {
		return "", fmt.Errorf("no URL found in service status output: %q", strings.TrimSpace(string(out)))
	}
	return u, nil
}

// DefaultPasswordPath is the service credentials file location.
func DefaultPasswordPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "service.json")
}

// PasswordFromFile reads the Basic-auth password from service.json.
func PasswordFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read service.json: %w", err)
	}
	var f struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return "", fmt.Errorf("parse service.json: %w", err)
	}
	if f.Password == "" {
		return "", fmt.Errorf("service.json has no password")
	}
	return f.Password, nil
}

// DiscoverClient discovers the service and returns an authenticated client.
func DiscoverClient(ctx context.Context) (*Client, error) {
	base, err := Discover(ctx)
	if err != nil {
		return nil, err
	}
	pw, err := PasswordFromFile(DefaultPasswordPath())
	if err != nil {
		return nil, err
	}
	c := New(base, pw)
	if err := c.Healthy(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// APIError is a non-2xx response from the service.
type APIError struct {
	StatusCode int
	Tag        string
	Message    string
}

func (e *APIError) Error() string {
	if e.Tag != "" {
		return fmt.Sprintf("opencode API %d: %s: %s", e.StatusCode, e.Tag, e.Message)
	}
	return fmt.Sprintf("opencode API %d: %s", e.StatusCode, e.Message)
}

// do executes an authenticated request. body may be nil. On 2xx it
// unmarshals the {"data": ...} envelope into out (when out != nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		rdr = strings.NewReader(string(b))
	} else {
		rdr = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.SetBasicAuth("opencode", c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Tag     string `json:"_tag"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Message == "" {
			e.Message = http.StatusText(resp.StatusCode)
		}
		return &APIError{StatusCode: resp.StatusCode, Tag: e.Tag, Message: e.Message}
	}
	if out == nil {
		return nil
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(env.Data) == 0 {
		return fmt.Errorf("response missing data envelope")
	}
	return json.Unmarshal(env.Data, out)
}

// Healthy verifies the service responds to an authenticated health check.
func (c *Client) Healthy(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/health", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("opencode", c.password)
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: status %d (auth or service problem)", resp.StatusCode)
	}
	return nil
}

// Session is an opencode session as returned by the API. ParentID is set on
// subagent child sessions and names the session that spawned them.
type Session struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Agent    string `json:"agent"`
	ParentID string `json:"parentID,omitempty"`
	Location struct {
		Directory string `json:"directory"`
	} `json:"location"`
	Tokens struct {
		Input  int `json:"input"`
		Output int `json:"output"`
	} `json:"tokens"`
	Time struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// CreateSession creates a session titled title scoped to directory.
func (c *Client) CreateSession(ctx context.Context, title, directory string) (*Session, error) {
	return c.CreateSessionWith(ctx, title, directory, "", nil)
}

// ModelRef identifies a provider model, per the V2 Model.Ref schema. Model
// IDs may contain slashes; ProviderID is required alongside ID. Variant is
// optional and currently never set by lessmess.
type ModelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Variant    string `json:"variant,omitempty"`
}

// CreateSessionWith is CreateSession with optional agent and model
// defaults applied at creation (the V2 create endpoint accepts both).
// Empty agent or a nil/incomplete model leaves the service default in
// place. The service answers 400 for an unknown agent/model — callers
// decide whether to retry plainly.
func (c *Client) CreateSessionWith(ctx context.Context, title, directory, agent string, model *ModelRef) (*Session, error) {
	body := map[string]any{
		"title":    title,
		"location": map[string]string{"directory": directory},
	}
	if agent != "" {
		body["agent"] = agent
	}
	if model != nil && model.ID != "" && model.ProviderID != "" {
		body["model"] = model
	}
	var s Session
	if err := c.do(ctx, http.MethodPost, "/api/session", body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// AgentInfo is one registered agent as returned by GET /api/agent. Mode is
// "primary" (usable as a session driver) or "subagent".
type AgentInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	Hidden      bool   `json:"hidden"`
}

// ModelInfo is one available model as returned by GET /api/model.
type ModelInfo struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Name       string `json:"name"`
}

// ListAgents returns the service's registered agents for its default
// location.
func (c *Client) ListAgents(ctx context.Context) ([]AgentInfo, error) {
	return c.ListAgentsFor(ctx, "")
}

// ListAgentsFor is ListAgents scoped to a repository directory: agents
// defined by that project's own opencode configuration are included. An
// empty dir uses the service default location.
func (c *Client) ListAgentsFor(ctx context.Context, dir string) ([]AgentInfo, error) {
	var agents []AgentInfo
	if err := c.do(ctx, http.MethodGet, "/api/agent"+locationQuery(dir), nil, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// ListModels returns the models available to the service's default
// location.
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return c.ListModelsFor(ctx, "")
}

// ListModelsFor is ListModels scoped to a repository directory (providers
// can be project-configured). An empty dir uses the default location.
func (c *Client) ListModelsFor(ctx context.Context, dir string) ([]ModelInfo, error) {
	var models []ModelInfo
	if err := c.do(ctx, http.MethodGet, "/api/model"+locationQuery(dir), nil, &models); err != nil {
		return nil, err
	}
	return models, nil
}

// locationQuery builds the deepObject location query the V2 API expects:
// ?location[directory]=<dir>, empty for the default location.
func locationQuery(dir string) string {
	if dir == "" {
		return ""
	}
	return "?location[directory]=" + url.QueryEscape(dir)
}

// DefaultModel returns the service's default model.
func (c *Client) DefaultModel(ctx context.Context) (*ModelInfo, error) {
	var m ModelInfo
	if err := c.do(ctx, http.MethodGet, "/api/model/default", nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// GetSession returns one session.
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	var s Session
	if err := c.do(ctx, http.MethodGet, "/api/session/"+id, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSessions returns all sessions known to the service.
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var ss []Session
	if err := c.do(ctx, http.MethodGet, "/api/session", nil, &ss); err != nil {
		return nil, err
	}
	return ss, nil
}

// RenameSession sets a session's title.
func (c *Client) RenameSession(ctx context.Context, id, title string) error {
	return c.do(ctx, http.MethodPost, "/api/session/"+id+"/rename", map[string]string{"title": title}, nil)
}

// DeleteSession deletes a session.
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/session/"+id, nil, nil)
}

// Prompt sends a text message to a session.
func (c *Client) Prompt(ctx context.Context, id, text string) error {
	return c.do(ctx, http.MethodPost, "/api/session/"+id+"/prompt", map[string]string{"text": text}, nil)
}

// WaitDone blocks until the session is idle (POST wait returns 204) or
// ctx expires. A nil return means the session is done/idle. The service's
// wait endpoint blocks server-side, often longer than the HTTP client's own
// 30s cap; those transport-level timeouts are retried until ctx expires.
func (c *Client) WaitDone(ctx context.Context, id string) error {
	for {
		err := c.do(ctx, http.MethodPost, "/api/session/"+id+"/wait", nil, nil)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err // caller's ctx expired: session still busy (or dead)
		}
		if !isTimeoutErr(err) {
			return err
		}
		// Transport timeout: the wait is still pending server-side; re-issue.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// isTimeoutErr reports whether err is a transport-level timeout (the HTTP
// client's own cap or a net timeout), as opposed to a server error. It must
// look through the fmt wrapping in do, so os.IsTimeout is insufficient.
func isTimeoutErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
