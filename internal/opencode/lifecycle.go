package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// SessionRevert is the staged revert state embedded in Session.Info.
type SessionRevert struct {
	MessageID string     `json:"messageID"`
	PartID    string     `json:"partID,omitempty"`
	Files     []FileDiff `json:"files,omitempty"`
}

// ShellOutput is bounded by the server before it is shown to a browser.
type ShellOutput struct {
	Output    string `json:"output"`
	Cursor    int    `json:"cursor"`
	Size      int    `json:"size"`
	Truncated bool   `json:"truncated"`
}

type Delivery string

const (
	DeliveryQueue Delivery = "queue"
	DeliverySteer Delivery = "steer"
)

// InboxItem is the display-safe common subset of all inbox variants. Unknown
// variants retain only their discriminator, never their raw payload.
type InboxItem struct {
	ID          string   `json:"id"`
	SessionID   string   `json:"sessionID"`
	Type        string   `json:"type"`
	Delivery    Delivery `json:"delivery"`
	Text        string   `json:"text,omitempty"`
	Description string   `json:"description,omitempty"`
	Created     float64  `json:"created,omitempty"`
	Known       bool     `json:"known"`
}

func (i *InboxItem) UnmarshalJSON(data []byte) error {
	*i = InboxItem{}
	var wire struct {
		ID, SessionID, Type string
		Delivery            Delivery `json:"delivery"`
		TimeCreated         float64  `json:"timeCreated"`
		Time                struct {
			Created float64 `json:"created"`
		} `json:"time"`
		Payload struct{ Text, Description string } `json:"payload"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	i.ID, i.SessionID, i.Type, i.Delivery = wire.ID, wire.SessionID, wire.Type, wire.Delivery
	i.Created = wire.TimeCreated
	if i.Created == 0 {
		i.Created = wire.Time.Created
	}
	switch i.Type {
	case "user", "synthetic", "compaction", "move":
		i.Known = true
		i.Text, i.Description = wire.Payload.Text, wire.Payload.Description
	}
	return nil
}

type SessionPage struct {
	Sessions []Session `json:"sessions"`
	Cursor   Cursor    `json:"cursor"`
}

type LifecycleCapabilities struct {
	Fork                                   bool   `json:"fork"`
	RevertStage                            bool   `json:"revertStage"`
	RevertCommit                           bool   `json:"revertCommit"`
	RevertClear                            bool   `json:"revertClear"`
	Compact                                bool   `json:"compact"`
	InboxList                              bool   `json:"inboxList"`
	InboxDelivery                          bool   `json:"inboxDelivery"`
	InboxCancel                            bool   `json:"inboxCancel"`
	PromptDelivery                         bool   `json:"promptDelivery"`
	PromptDeliveryRich                     bool   `json:"promptDeliveryRich"`
	PromptDeliveryFiles                    bool   `json:"promptDeliveryFiles"`
	PromptDeliverySkills                   bool   `json:"promptDeliverySkills"`
	PromptDeliveryID                       bool   `json:"promptDeliveryID"`
	Rename                                 bool   `json:"rename"`
	Export                                 bool   `json:"export"`
	Delete                                 bool   `json:"delete"`
	ForkBoundary                           bool   `json:"-"`
	RevertClearMethod, RevertClearPath     string `json:"-"`
	InboxDeliveryMethod, InboxDeliveryPath string `json:"-"`
	RenameMethod, RenamePath               string `json:"-"`
}

type openAPISpec struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func (c *Client) openAPI(ctx context.Context) (openAPISpec, error) {
	var spec openAPISpec
	base, password := c.connection()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/openapi.json", nil)
	if err != nil {
		return spec, err
	}
	req.SetBasicAuth("opencode", password)
	resp, err := c.hc.Do(req)
	if err != nil {
		return spec, fmt.Errorf("GET /openapi.json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return spec, errors.New("OpenCode capabilities unavailable")
	}
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return spec, errors.New("OpenCode capabilities unavailable")
	}
	return spec, nil
}

func hasOperation(spec openAPISpec, method, path string) bool {
	return spec.Paths[path] != nil && spec.Paths[path][strings.ToLower(method)] != nil
}

// LifecycleCapabilities detects exact methods and paths. It does not infer
// behavior from a service version or probe any mutating endpoint.
func (c *Client) LifecycleCapabilities(ctx context.Context) (LifecycleCapabilities, error) {
	spec, err := c.openAPI(ctx)
	if err != nil {
		return LifecycleCapabilities{}, err
	}
	has := func(method, path string) bool { return hasOperation(spec, method, path) }
	cap := LifecycleCapabilities{
		Fork:         has(http.MethodPost, "/api/session/{sessionID}/fork"),
		RevertStage:  has(http.MethodPost, "/api/session/{sessionID}/revert/stage"),
		RevertCommit: has(http.MethodPost, "/api/session/{sessionID}/revert/commit"),
		Compact:      has(http.MethodPost, "/api/session/{sessionID}/compact"),
		InboxList:    has(http.MethodGet, "/api/session/{sessionID}/inbox"),
		InboxCancel:  has(http.MethodDelete, "/api/session/{sessionID}/inbox/{inboxID}"),
		Export:       has(http.MethodGet, "/api/experimental/session/{sessionID}/export"),
		Delete:       has(http.MethodDelete, "/api/session/{sessionID}"),
	}
	if raw := spec.Paths["/api/session/{sessionID}/fork"]["post"]; strings.Contains(string(raw), `"boundary"`) {
		cap.ForkBoundary = true
	}
	if raw := spec.Paths["/api/session/{sessionID}/prompt"]["post"]; raw != nil {
		contract := string(raw)
		cap.PromptDelivery = strings.Contains(contract, `"delivery"`)
		cap.PromptDeliveryID = strings.Contains(contract, `"id"`)
		cap.PromptDeliveryFiles = strings.Contains(contract, `"files"`)
		cap.PromptDeliverySkills = strings.Contains(contract, `"skills"`)
		cap.PromptDeliveryRich = cap.PromptDeliveryFiles && cap.PromptDeliverySkills
	}
	if has(http.MethodPatch, "/api/session/{sessionID}") {
		cap.Rename, cap.RenameMethod, cap.RenamePath = true, http.MethodPatch, "/api/session/{sessionID}"
	} else if has(http.MethodPost, "/api/session/{sessionID}/rename") {
		cap.Rename, cap.RenameMethod, cap.RenamePath = true, http.MethodPost, "/api/session/{sessionID}/rename"
	}
	if has(http.MethodDelete, "/api/session/{sessionID}/revert") {
		cap.RevertClear, cap.RevertClearMethod, cap.RevertClearPath = true, http.MethodDelete, "/api/session/{sessionID}/revert"
	} else if has(http.MethodPost, "/api/session/{sessionID}/revert/clear") {
		cap.RevertClear, cap.RevertClearMethod, cap.RevertClearPath = true, http.MethodPost, "/api/session/{sessionID}/revert/clear"
	}
	if has(http.MethodPatch, "/api/session/{sessionID}/inbox/{inboxID}") {
		cap.InboxDelivery, cap.InboxDeliveryMethod, cap.InboxDeliveryPath = true, http.MethodPatch, "/api/session/{sessionID}/inbox/{inboxID}"
	} else if has(http.MethodPost, "/api/session/{sessionID}/inbox/{inboxID}/queue") && has(http.MethodPost, "/api/session/{sessionID}/inbox/{inboxID}/steer") {
		cap.InboxDelivery, cap.InboxDeliveryMethod, cap.InboxDeliveryPath = true, http.MethodPost, "/api/session/{sessionID}/inbox/{inboxID}/{delivery}"
	}
	return cap, nil
}

var ErrCapabilityUnavailable = errors.New("OpenCode capability unavailable")

func unavailable(ok bool, operation string) error {
	if !ok {
		return fmt.Errorf("%w: %s", ErrCapabilityUnavailable, operation)
	}
	return nil
}

func (c *Client) ForkSession(ctx context.Context, cap LifecycleCapabilities, id, before string) (*Session, error) {
	if err := unavailable(cap.Fork, "session fork"); err != nil {
		return nil, err
	}
	var body any = map[string]string{"before": before}
	if cap.ForkBoundary {
		body = map[string]any{"boundary": map[string]string{"type": "before", "messageID": before}}
	}
	var session Session
	if err := c.do(ctx, http.MethodPost, "/api/session/"+id+"/fork", body, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *Client) StageRevert(ctx context.Context, cap LifecycleCapabilities, id, messageID string, files bool) error {
	if err := unavailable(cap.RevertStage, "revert staging"); err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/api/session/"+id+"/revert/stage", map[string]any{"messageID": messageID, "files": files}, nil)
}
func (c *Client) CommitRevert(ctx context.Context, cap LifecycleCapabilities, id string) error {
	if err := unavailable(cap.RevertCommit, "revert commit"); err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/api/session/"+id+"/revert/commit", nil, nil)
}
func (c *Client) ClearRevert(ctx context.Context, cap LifecycleCapabilities, id string) error {
	if err := unavailable(cap.RevertClear, "revert clear"); err != nil {
		return err
	}
	return c.do(ctx, cap.RevertClearMethod, strings.Replace(cap.RevertClearPath, "{sessionID}", id, 1), nil, nil)
}
func (c *Client) CompactSession(ctx context.Context, cap LifecycleCapabilities, id, messageID string, delivery Delivery) (*InboxItem, error) {
	if err := unavailable(cap.Compact, "manual compaction"); err != nil {
		return nil, err
	}
	var item InboxItem
	if err := c.do(ctx, http.MethodPost, "/api/session/"+id+"/compact", map[string]any{"id": messageID, "delivery": delivery}, &item); err != nil {
		return nil, err
	}
	return &item, nil
}
func (c *Client) DeliverPrompt(ctx context.Context, cap LifecycleCapabilities, id, messageID, text string, files []PromptFile, skillIDs []string, delivery Delivery) (*InboxItem, error) {
	if err := unavailable(cap.PromptDelivery, "prompt delivery"); err != nil {
		return nil, err
	}
	if err := unavailable(cap.PromptDeliveryID, "idempotent prompt delivery"); err != nil {
		return nil, err
	}
	if len(files) > 0 && !cap.PromptDeliveryFiles {
		return nil, fmt.Errorf("%w: prompt delivery with attachments", ErrCapabilityUnavailable)
	}
	if len(skillIDs) > 0 && !cap.PromptDeliverySkills {
		return nil, fmt.Errorf("%w: prompt delivery with skills", ErrCapabilityUnavailable)
	}
	type skillAttachment struct {
		ID string `json:"id"`
	}
	body := struct {
		ID       string            `json:"id"`
		Text     string            `json:"text"`
		Files    []PromptFile      `json:"files,omitempty"`
		Skills   []skillAttachment `json:"skills,omitempty"`
		Delivery Delivery          `json:"delivery"`
	}{ID: messageID, Text: text, Files: files, Delivery: delivery}
	for _, skillID := range skillIDs {
		body.Skills = append(body.Skills, skillAttachment{ID: skillID})
	}
	var item InboxItem
	if err := c.do(ctx, http.MethodPost, "/api/session/"+id+"/prompt", body, &item); err != nil {
		return nil, err
	}
	return &item, nil
}
func (c *Client) ListInbox(ctx context.Context, cap LifecycleCapabilities, id string) ([]InboxItem, error) {
	if err := unavailable(cap.InboxList, "session inbox"); err != nil {
		return nil, err
	}
	var items []InboxItem
	if err := c.do(ctx, http.MethodGet, "/api/session/"+id+"/inbox", nil, &items); err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(a, b int) bool {
		if items[a].Created == items[b].Created {
			return items[a].ID < items[b].ID
		}
		return items[a].Created < items[b].Created
	})
	return items, nil
}
func (c *Client) ChangeInboxDelivery(ctx context.Context, cap LifecycleCapabilities, sessionID, inboxID string, delivery Delivery) error {
	if err := unavailable(cap.InboxDelivery, "inbox delivery update"); err != nil {
		return err
	}
	path := strings.ReplaceAll(cap.InboxDeliveryPath, "{sessionID}", sessionID)
	path = strings.ReplaceAll(path, "{inboxID}", inboxID)
	path = strings.ReplaceAll(path, "{delivery}", string(delivery))
	var body any
	if cap.InboxDeliveryMethod == http.MethodPatch {
		body = map[string]Delivery{"delivery": delivery}
	}
	return c.do(ctx, cap.InboxDeliveryMethod, path, body, nil)
}
func (c *Client) CancelInbox(ctx context.Context, cap LifecycleCapabilities, sessionID, inboxID string) error {
	if err := unavailable(cap.InboxCancel, "inbox cancellation"); err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, "/api/session/"+sessionID+"/inbox/"+inboxID, nil, nil)
}

// RenameSessionCompatible selects the exact advertised update route. Published
// services use PATCH while the installed beta retains POST /rename.
func (c *Client) RenameSessionCompatible(ctx context.Context, cap LifecycleCapabilities, id, title string) error {
	if err := unavailable(cap.Rename, "session rename"); err != nil {
		return err
	}
	path := strings.Replace(cap.RenamePath, "{sessionID}", id, 1)
	return c.do(ctx, cap.RenameMethod, path, map[string]string{"title": title}, nil)
}

func (c *Client) ListChildrenPage(ctx context.Context, parentID string, limit int, cursor string) (SessionPage, error) {
	q := url.Values{"parentID": {parentID}, "limit": {strconv.Itoa(limit)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	} else {
		q.Set("order", "asc")
	}
	env, err := c.doEnvelope(ctx, http.MethodGet, "/api/session?"+q.Encode(), nil, true)
	var page SessionPage
	if err != nil {
		return page, err
	}
	if err := json.Unmarshal(env.Data, &page.Sessions); err != nil {
		return page, err
	}
	page.Cursor = env.Cursor
	return page, nil
}

type SessionExport struct {
	Info     Session   `json:"info"`
	Messages []Message `json:"messages"`
}

func (c *Client) ExportSessionSanitized(ctx context.Context, cap LifecycleCapabilities, id string) (*SessionExport, error) {
	if err := unavailable(cap.Export, "sanitized session export"); err != nil {
		return nil, err
	}
	var result SessionExport
	if err := c.do(ctx, http.MethodGet, "/api/experimental/session/"+id+"/export?sanitize=true", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
