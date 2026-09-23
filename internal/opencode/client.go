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
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client talks to one opencode service over HTTP Basic auth.
type Client struct {
	mu       sync.RWMutex
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
func (c *Client) BaseURL() string {
	base, _ := c.connection()
	return base
}

func (c *Client) connection() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base, c.password
}

// Rediscover validates the currently registered service and atomically moves
// this client to it. It does not start, stop, or restart the service.
func (c *Client) Rediscover(ctx context.Context) error {
	base, err := discoverService(ctx)
	if err != nil {
		return err
	}
	password, err := readServicePassword()
	if err != nil {
		return err
	}
	candidate := New(base, password)
	if _, err := candidate.ServiceIdentity(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	c.base, c.password = candidate.base, candidate.password
	c.mu.Unlock()
	return nil
}

var serviceURLRe = regexp.MustCompile(`https?://[^\s"']+`)

var discoverService = Discover
var readServicePassword = func() (string, error) { return PasswordFromFile(DefaultPasswordPath()) }

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
	env, err := c.doEnvelope(ctx, method, path, body, out != nil)
	if err != nil || out == nil {
		return err
	}
	if len(env.Data) == 0 {
		return fmt.Errorf("response missing data envelope")
	}
	return json.Unmarshal(env.Data, out)
}

type responseEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Cursor Cursor          `json:"cursor"`
}

func (c *Client) doEnvelope(ctx context.Context, method, path string, body any, decode bool) (responseEnvelope, error) {
	var env responseEnvelope
	var rdr *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return env, fmt.Errorf("marshal body: %w", err)
		}
		rdr = strings.NewReader(string(b))
	} else {
		rdr = strings.NewReader("")
	}
	base, password := c.connection()
	req, err := http.NewRequestWithContext(ctx, method, base+path, rdr)
	if err != nil {
		return env, err
	}
	req.SetBasicAuth("opencode", password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return env, fmt.Errorf("%s %s: %w", method, path, err)
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
		return env, &APIError{StatusCode: resp.StatusCode, Tag: e.Tag, Message: e.Message}
	}
	if !decode {
		return env, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return env, fmt.Errorf("decode response: %w", err)
	}
	return env, nil
}

// Healthy verifies the service responds to an authenticated health check.
func (c *Client) Healthy(ctx context.Context) error {
	_, err := c.ServiceIdentity(ctx)
	return err
}

// Session is an opencode session as returned by the API. ParentID is set on
// subagent child sessions and names the session that spawned them.
type Session struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Agent    string         `json:"agent"`
	Model    *ModelRef      `json:"model,omitempty"`
	ParentID string         `json:"parentID,omitempty"`
	Revert   *SessionRevert `json:"revert,omitempty"`
	Location struct {
		Directory string `json:"directory"`
	} `json:"location"`
	Tokens TokenUsage `json:"tokens"`
	Time   struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// Message is the stable subset of a projected session message used by chat.
// Type-specific fields that chat does not need are deliberately left out.
type Message struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	Text        string           `json:"text,omitempty"`
	Description string           `json:"description,omitempty"`
	Agent       string           `json:"agent,omitempty"`
	Model       *ModelRef        `json:"model,omitempty"`
	Content     []MessagePart    `json:"-"`
	Files       []FileAttachment `json:"files,omitempty"`
	Error       *StructuredError `json:"error,omitempty"`
	Finish      string           `json:"finish,omitempty"`
	Retry       *MessageRetry    `json:"retry,omitempty"`
	Status      string           `json:"status,omitempty"`
	Reason      string           `json:"reason,omitempty"`
	Summary     string           `json:"summary,omitempty"`
	ShellID     string           `json:"shellID,omitempty"`
	Command     string           `json:"command,omitempty"`
	Exit        json.RawMessage  `json:"exit,omitempty"`
	Output      *ShellOutput     `json:"output,omitempty"`
	Tokens      *TokenUsage      `json:"tokens,omitempty"`
	Time        MessageTime      `json:"time"`
	Raw         json.RawMessage  `json:"-"`
}

// TokenUsage is the accounting shape shared by sessions and assistant
// messages. Message usage describes one provider request; session usage is
// cumulative accounting across the session.
type TokenUsage struct {
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

// MessageRetry is the public retry status on an assistant message. The
// provider-specific state is intentionally not represented by this client.
type MessageRetry struct {
	Attempt int              `json:"attempt"`
	At      float64          `json:"at"`
	Error   *StructuredError `json:"error,omitempty"`
}

// FileAttachment is the projected form of a file included with a user prompt.
// Callers must not use Source.URI as authority to read a local file.
type FileAttachment struct {
	Data   string     `json:"data"`
	MIME   string     `json:"mime"`
	Name   string     `json:"name,omitempty"`
	Source FileSource `json:"source"`
}

type FileSource struct {
	Type string `json:"type"`
	URI  string `json:"uri,omitempty"`
}

// PromptFile is one URI accepted by the V2 prompt endpoint.
type PromptFile struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// Reference is one location-scoped project reference.
type Reference struct {
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	Description string          `json:"description,omitempty"`
	Hidden      bool            `json:"hidden,omitempty"`
	Source      json.RawMessage `json:"source"`
}

// MessageTime contains timestamps present across projected message variants.
type MessageTime struct {
	Created   float64 `json:"created"`
	Ran       float64 `json:"ran,omitempty"`
	Streamed  float64 `json:"streamed,omitempty"`
	Completed float64 `json:"completed,omitempty"`
}

// StructuredError is the displayable error shape used by assistant and tool
// messages.
type StructuredError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Status  int    `json:"status,omitempty"`
}

// MessagePart is one assistant content item. Implementations retain their raw
// JSON so newly added fields can be inspected without widening this adapter.
type MessagePart interface {
	PartType() string
	PartRaw() json.RawMessage
}

// TextPart is assistant prose.
type TextPart struct {
	Type string          `json:"type"`
	Text string          `json:"text"`
	Raw  json.RawMessage `json:"-"`
}

func (p TextPart) PartType() string         { return p.Type }
func (p TextPart) PartRaw() json.RawMessage { return p.Raw }

// ReasoningPart is assistant reasoning text.
type ReasoningPart struct {
	Type string          `json:"type"`
	Text string          `json:"text"`
	Time MessageTime     `json:"time,omitempty"`
	Raw  json.RawMessage `json:"-"`
}

func (p ReasoningPart) PartType() string         { return p.Type }
func (p ReasoningPart) PartRaw() json.RawMessage { return p.Raw }

// ToolPart describes a tool call and its current state.
type ToolPart struct {
	Type     string          `json:"type"`
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Executed bool            `json:"executed,omitempty"`
	State    ToolState       `json:"state"`
	Time     MessageTime     `json:"time"`
	Raw      json.RawMessage `json:"-"`
}

func (p ToolPart) PartType() string         { return p.Type }
func (p ToolPart) PartRaw() json.RawMessage { return p.Raw }

// ToolState is the common streaming/running/completed/error tool state.
type ToolState struct {
	Status   string           `json:"status"`
	Input    json.RawMessage  `json:"input,omitempty"`
	Content  []ToolContent    `json:"content,omitempty"`
	Error    *StructuredError `json:"error,omitempty"`
	Metadata json.RawMessage  `json:"metadata,omitempty"`
}

// ToolContent is the common text or file result returned by a tool. Raw is
// retained for future content variants.
type ToolContent struct {
	Type string          `json:"type"`
	Text string          `json:"text,omitempty"`
	URI  string          `json:"uri,omitempty"`
	MIME string          `json:"mime,omitempty"`
	Name string          `json:"name,omitempty"`
	Raw  json.RawMessage `json:"-"`
}

func (c *ToolContent) UnmarshalJSON(data []byte) error {
	type plain ToolContent
	if err := json.Unmarshal(data, (*plain)(c)); err != nil {
		return err
	}
	c.Raw = append(c.Raw[:0], data...)
	return nil
}

// UnknownPart safely preserves an assistant content variant unknown to this
// client. Callers should render only Type, not Raw, unless they escape it.
type UnknownPart struct {
	Type string
	Raw  json.RawMessage
}

func (p UnknownPart) PartType() string         { return p.Type }
func (p UnknownPart) PartRaw() json.RawMessage { return p.Raw }

func (m *Message) UnmarshalJSON(data []byte) error {
	type plain Message
	var wire struct {
		plain
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*m = Message(wire.plain)
	m.Raw = append(m.Raw[:0], data...)
	for _, raw := range wire.Content {
		var discriminator struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &discriminator); err != nil {
			return fmt.Errorf("decode message part: %w", err)
		}
		var part MessagePart
		switch discriminator.Type {
		case "text":
			var value TextPart
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode text part: %w", err)
			}
			value.Raw = append(value.Raw[:0], raw...)
			part = value
		case "reasoning":
			var value ReasoningPart
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode reasoning part: %w", err)
			}
			value.Raw = append(value.Raw[:0], raw...)
			part = value
		case "tool":
			var value ToolPart
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode tool part: %w", err)
			}
			value.Raw = append(value.Raw[:0], raw...)
			part = value
		default:
			part = UnknownPart{Type: discriminator.Type, Raw: append(json.RawMessage(nil), raw...)}
		}
		m.Content = append(m.Content, part)
	}
	return nil
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
	ID         string         `json:"id"`
	ProviderID string         `json:"providerID"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Enabled    *bool          `json:"enabled,omitempty"`
	Variants   []ModelVariant `json:"variants,omitempty"`
	Limit      struct {
		Context int `json:"context"`
		Input   int `json:"input"`
		Output  int `json:"output"`
	} `json:"limit"`
}

// ModelVariant is one selectable settings profile advertised by a model.
// The service applies the profile; lessmess only needs its public ID.
type ModelVariant struct {
	ID string `json:"id"`
}

// IsEnabled accepts responses from older services that did not emit enabled,
// while excluding models explicitly disabled by current services.
func (m ModelInfo) IsEnabled() bool { return m.Enabled == nil || *m.Enabled }

// CommandInfo is the display-safe command metadata needed by Chat.
type CommandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SkillInfo deliberately excludes skill content and filesystem location.
type SkillInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
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

// ListCommandsFor lists commands resolved for a project location.
func (c *Client) ListCommandsFor(ctx context.Context, dir string) ([]CommandInfo, error) {
	var commands []CommandInfo
	if err := c.do(ctx, http.MethodGet, "/api/command"+locationQuery(dir), nil, &commands); err != nil {
		return nil, err
	}
	return commands, nil
}

// ListSkillsFor lists only display-safe skill metadata for a project location.
func (c *Client) ListSkillsFor(ctx context.Context, dir string) ([]SkillInfo, error) {
	var skills []SkillInfo
	if err := c.do(ctx, http.MethodGet, "/api/skill"+locationQuery(dir), nil, &skills); err != nil {
		return nil, err
	}
	return skills, nil
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
	var m *ModelInfo
	if err := c.do(ctx, http.MethodGet, "/api/model/default", nil, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetSession returns one session.
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	var s Session
	if err := c.do(ctx, http.MethodGet, "/api/session/"+id, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSessionContext returns the active messages after the session's latest
// compaction boundary.
func (c *Client) GetSessionContext(ctx context.Context, id string) ([]Message, error) {
	var messages []Message
	if err := c.do(ctx, http.MethodGet, "/api/session/"+id+"/context", nil, &messages); err != nil {
		return nil, err
	}
	return messages, nil
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
	return c.do(ctx, http.MethodPatch, "/api/session/"+id, map[string]string{"title": title}, nil)
}

// DeleteSession deletes a session.
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/session/"+id, nil, nil)
}

// Prompt sends a text message to a session.
func (c *Client) Prompt(ctx context.Context, id, text string) error {
	return c.do(ctx, http.MethodPost, "/api/session/"+id+"/prompt", map[string]string{"text": text}, nil)
}

// PromptWithFiles sends text and the narrow file attachment shape used by chat.
func (c *Client) PromptWithFiles(ctx context.Context, id, text string, files []PromptFile) error {
	return c.PromptWithFilesAndSkills(ctx, id, text, files, nil)
}

// PromptWithFilesAndSkills sends prompt-time skill attachments by public ID.
func (c *Client) PromptWithFilesAndSkills(ctx context.Context, id, text string, files []PromptFile, skillIDs []string) error {
	type skillAttachment struct {
		ID string `json:"id"`
	}
	body := struct {
		Text   string            `json:"text"`
		Files  []PromptFile      `json:"files,omitempty"`
		Skills []skillAttachment `json:"skills,omitempty"`
	}{Text: text, Files: files}
	for _, id := range skillIDs {
		body.Skills = append(body.Skills, skillAttachment{ID: id})
	}
	return c.do(ctx, http.MethodPost, "/api/session/"+id+"/prompt", body, nil)
}

// SwitchAgent changes the agent used by subsequent session turns.
func (c *Client) SwitchAgent(ctx context.Context, sessionID, agent string) error {
	return c.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/agent", map[string]string{"agent": agent}, nil)
}

// SwitchModel changes the model used by subsequent session turns.
func (c *Client) SwitchModel(ctx context.Context, sessionID string, model ModelRef) error {
	return c.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/model", map[string]ModelRef{"model": model}, nil)
}

// RunCommand resolves and executes one registered command.
func (c *Client) RunCommand(ctx context.Context, sessionID, command, arguments string) error {
	body := struct {
		Command   string `json:"command"`
		Arguments string `json:"arguments,omitempty"`
	}{Command: command, Arguments: arguments}
	return c.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/command", body, nil)
}

// StandaloneSkillRoute capability-detects the exact current or beta route.
// It never probes by activating a skill.
func (c *Client) StandaloneSkillRoute(ctx context.Context) (string, bool, error) {
	base, password := c.connection()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/openapi.json", nil)
	if err != nil {
		return "", false, err
	}
	req.SetBasicAuth("opencode", password)
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("GET /openapi.json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, nil
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return "", false, nil
	}
	for _, route := range []string{"/api/experimental/session/{sessionID}/skill", "/api/session/{sessionID}/skill"} {
		if methods := spec.Paths[route]; methods != nil && methods[strings.ToLower(http.MethodPost)] != nil {
			return route, true, nil
		}
	}
	return "", false, nil
}

// ActivateSkill uses only a route previously capability-detected from OpenAPI.
func (c *Client) ActivateSkill(ctx context.Context, route, sessionID, skill string) error {
	if route != "/api/experimental/session/{sessionID}/skill" && route != "/api/session/{sessionID}/skill" {
		return errors.New("standalone skill activation unavailable")
	}
	path := strings.Replace(route, "{sessionID}", sessionID, 1)
	return c.do(ctx, http.MethodPost, path, map[string]string{"skill": skill}, nil)
}

// ListReferencesFor lists references resolved in the supplied project.
func (c *Client) ListReferencesFor(ctx context.Context, dir string) ([]Reference, error) {
	var refs []Reference
	if err := c.do(ctx, http.MethodGet, "/api/reference"+locationQuery(dir), nil, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

// Cursor contains opaque tokens for adjacent pages in the requested ordering.
type Cursor struct {
	Previous string `json:"previous"`
	Next     string `json:"next"`
}

// MessagePage is one projected transcript page and its top-level cursor.
type MessagePage struct {
	Messages []Message
	Cursor   Cursor
}

// ListMessagesOptions controls one message page request. Cursor must not be
// combined with Order; the service retains the original order in cursor tokens.
type ListMessagesOptions struct {
	Limit  int
	Order  string
	Cursor string
}

// ListMessages returns the service-default transcript page for compatibility.
func (c *Client) ListMessages(ctx context.Context, id string) ([]Message, error) {
	page, err := c.ListMessagesPage(ctx, id, ListMessagesOptions{})
	if err != nil {
		return nil, err
	}
	return page.Messages, nil
}

// GetMessage returns one projected session message.
func (c *Client) GetMessage(ctx context.Context, sessionID, messageID string) (*Message, error) {
	var message Message
	if err := c.do(ctx, http.MethodGet, "/api/session/"+sessionID+"/message/"+messageID, nil, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

// FileDiff is one structured per-file change returned for a session turn.
type FileDiff struct {
	File      string `json:"file"`
	Patch     string `json:"patch"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"`
}

// GetSessionDiff returns the structured diff from one user-message turn through
// an optional later user-message turn. Context is the number of unchanged lines.
func (c *Client) GetSessionDiff(ctx context.Context, sessionID, from, to string, contextLines int) ([]FileDiff, error) {
	query := url.Values{}
	if from != "" {
		query.Set("from", from)
	}
	if to != "" {
		query.Set("to", to)
	}
	if contextLines >= 0 {
		query.Set("context", strconv.Itoa(contextLines))
	}
	var diffs []FileDiff
	path := "/api/session/" + sessionID + "/diff?" + query.Encode()
	if err := c.do(ctx, http.MethodGet, path, nil, &diffs); err != nil {
		return nil, err
	}
	return diffs, nil
}

// ListMessagesPage returns one message page while preserving the response's
// top-level cursor. Callers pass cursor values through without interpreting them.
func (c *Client) ListMessagesPage(ctx context.Context, id string, opts ListMessagesOptions) (MessagePage, error) {
	var page MessagePage
	query := url.Values{}
	if opts.Limit > 0 {
		query.Set("limit", fmt.Sprint(opts.Limit))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	} else if opts.Order != "" {
		query.Set("order", opts.Order)
	}
	path := "/api/session/" + id + "/message"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	env, err := c.doEnvelope(ctx, http.MethodGet, path, nil, true)
	if err != nil {
		return page, err
	}
	if len(env.Data) == 0 {
		return page, fmt.Errorf("response missing data envelope")
	}
	if err := json.Unmarshal(env.Data, &page.Messages); err != nil {
		return page, err
	}
	page.Cursor = env.Cursor
	return page, nil
}

// SessionActive describes a foreground session execution. Sessions absent
// from ListActiveSessions are inactive.
type SessionActive struct {
	Type string `json:"type"`
}

// ListActiveSessions returns foreground executions owned by the service.
func (c *Client) ListActiveSessions(ctx context.Context) (map[string]SessionActive, error) {
	var active map[string]SessionActive
	if err := c.do(ctx, http.MethodGet, "/api/session/active", nil, &active); err != nil {
		return nil, err
	}
	return active, nil
}

// Interrupt stops active session execution and reports whether anything was
// interrupted. Interrupting an idle session is a successful no-op.
func (c *Client) Interrupt(ctx context.Context, id string) (bool, error) {
	var response struct {
		Interrupted bool `json:"interrupted"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/session/"+id+"/interrupt", nil, &response); err != nil {
		return false, err
	}
	return response.Interrupted, nil
}

// PermissionRequest is a pending decision owned by a session.
type PermissionRequest struct {
	ID        string           `json:"id"`
	SessionID string           `json:"sessionID"`
	Action    string           `json:"action"`
	Resources []string         `json:"resources"`
	Save      []string         `json:"save,omitempty"`
	Metadata  json.RawMessage  `json:"metadata,omitempty"`
	Source    PermissionSource `json:"source,omitempty"`
	Message   string           `json:"message,omitempty"`
}

// PermissionSource identifies the tool call that requested permission.
type PermissionSource struct {
	Type      string `json:"type"`
	MessageID string `json:"messageID"`
	ID        string `json:"id"`
}

// PermissionDecision is one of the decisions accepted by the V2 API.
type PermissionDecision string

const (
	PermissionOnce   PermissionDecision = "once"
	PermissionAlways PermissionDecision = "always"
	PermissionReject PermissionDecision = "reject"
)

// ListPermissions returns a session's pending permission requests.
func (c *Client) ListPermissions(ctx context.Context, sessionID string) ([]PermissionRequest, error) {
	var requests []PermissionRequest
	if err := c.do(ctx, http.MethodGet, "/api/session/"+sessionID+"/permission", nil, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

// ReplyPermission resolves a pending permission request. Message is optional.
func (c *Client) ReplyPermission(ctx context.Context, sessionID, requestID string, decision PermissionDecision, message string) error {
	body := struct {
		Decision PermissionDecision `json:"decision"`
		Message  string             `json:"message,omitempty"`
	}{Decision: decision, Message: message}
	return c.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/permission/"+requestID+"/reply", body, nil)
}

// Form is a pending structured form for a session.
type Form struct {
	ID        string          `json:"id"`
	SessionID string          `json:"sessionID"`
	Title     string          `json:"title"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	Fields    []FormField     `json:"fields"`
}

// FormField contains the common fields needed to render V2 string, number,
// integer, boolean, multiselect, and external fields. Variant-specific values
// remain raw because the schema is evolving.
type FormField struct {
	Key         string          `json:"key"`
	Type        string          `json:"type"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Required    bool            `json:"required,omitempty"`
	Hidden      bool            `json:"hidden,omitempty"`
	URL         string          `json:"url,omitempty"`
	Placeholder string          `json:"placeholder,omitempty"`
	Options     []FormOption    `json:"options,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
	Minimum     json.RawMessage `json:"minimum,omitempty"`
	Maximum     json.RawMessage `json:"maximum,omitempty"`
	MinLength   int             `json:"minLength,omitempty"`
	MaxLength   int             `json:"maxLength,omitempty"`
	MinItems    int             `json:"minItems,omitempty"`
	MaxItems    int             `json:"maxItems,omitempty"`
	Pattern     string          `json:"pattern,omitempty"`
	Format      string          `json:"format,omitempty"`
	Custom      bool            `json:"custom,omitempty"`
}

// FormOption is one selectable string value.
type FormOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// FormAnswer maps field keys to V2 form values (string, number, boolean, or a
// string slice). OpenCode validates values against the pending form.
type FormAnswer map[string]any

// ListForms returns a session's pending forms.
func (c *Client) ListForms(ctx context.Context, sessionID string) ([]Form, error) {
	var forms []Form
	if err := c.do(ctx, http.MethodGet, "/api/session/"+sessionID+"/form", nil, &forms); err != nil {
		return nil, err
	}
	return forms, nil
}

// ReplyForm submits an answer to a pending form.
func (c *Client) ReplyForm(ctx context.Context, sessionID, formID string, answer FormAnswer) error {
	body := struct {
		Answer FormAnswer `json:"answer"`
	}{Answer: answer}
	return c.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/form/"+formID+"/reply", body, nil)
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
