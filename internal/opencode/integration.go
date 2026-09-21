package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// IntegrationInfo is the display-safe integration projection. Metadata,
// command argv, provider settings, and credential values are never retained.
type IntegrationInfo struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Methods     []IntegrationMethod     `json:"methods"`
	Connections []IntegrationConnection `json:"connections"`
}

type IntegrationMethod struct {
	ID     string      `json:"id,omitempty"`
	Type   string      `json:"type"`
	Label  string      `json:"label,omitempty"`
	Fields []FormField `json:"fields,omitempty"`
	Names  []string    `json:"names,omitempty"`
}

type IntegrationConnection struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
	Name  string `json:"name,omitempty"`
}

func safeExternalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}

func (i *IntegrationInfo) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID          string            `json:"id"`
		Name        string            `json:"name"`
		Methods     []json.RawMessage `json:"methods"`
		Connections []json.RawMessage `json:"connections"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	i.ID, i.Name = wire.ID, wire.Name
	i.Methods = make([]IntegrationMethod, 0, len(wire.Methods))
	for _, raw := range wire.Methods {
		var method struct {
			ID, Type, Label string
			Form            []json.RawMessage `json:"form"`
			Names           []string          `json:"names"`
		}
		if json.Unmarshal(raw, &method) != nil {
			continue
		}
		projected := IntegrationMethod{ID: method.ID, Type: knownEnum(method.Type, "key", "oauth", "command", "env"), Label: method.Label}
		if projected.Type == "env" {
			projected.Names = append([]string(nil), method.Names...)
		}
		for _, fieldRaw := range method.Form {
			var field struct {
				Key, Type, Title, Description, URL string
				Required                           bool
				Options                            []FormOption
			}
			if json.Unmarshal(fieldRaw, &field) != nil || field.Key == "" {
				continue
			}
			projected.Fields = append(projected.Fields, FormField{Key: field.Key, Type: knownEnum(field.Type, "string", "number", "integer", "boolean", "multiselect", "external"), Title: field.Title, Description: field.Description, Required: field.Required, Options: field.Options, URL: safeExternalURL(field.URL)})
		}
		i.Methods = append(i.Methods, projected)
	}
	i.Connections = make([]IntegrationConnection, 0, len(wire.Connections))
	for _, raw := range wire.Connections {
		var connection IntegrationConnection
		if json.Unmarshal(raw, &connection) != nil {
			continue
		}
		connection.Type = knownEnum(connection.Type, "credential", "env")
		if connection.Type == "credential" {
			connection.Name = ""
		} else if connection.Type == "env" {
			connection.ID, connection.Label = "", ""
		} else {
			connection.ID, connection.Label, connection.Name = "", "", ""
		}
		i.Connections = append(i.Connections, connection)
	}
	return nil
}

func integrationPath(id string) string { return "/api/integration/" + url.PathEscape(id) }

func (c *Client) ListIntegrations(ctx context.Context, dir string) ([]IntegrationInfo, error) {
	var integrations []IntegrationInfo
	if err := c.do(ctx, http.MethodGet, "/api/integration"+locationQuery(dir), nil, &integrations); err != nil {
		return nil, err
	}
	return integrations, nil
}

func (c *Client) GetIntegration(ctx context.Context, dir, id string) (IntegrationInfo, error) {
	var integration IntegrationInfo
	if err := c.do(ctx, http.MethodGet, integrationPath(id)+locationQuery(dir), nil, &integration); err != nil {
		return IntegrationInfo{}, err
	}
	return integration, nil
}

type ConnectKeyInput struct {
	Key    string         `json:"key"`
	Answer map[string]any `json:"answer,omitempty"`
	Label  string         `json:"label,omitempty"`
}

type ConnectMethodInput struct {
	MethodID string         `json:"methodID"`
	Answer   map[string]any `json:"answer,omitempty"`
	Label    string         `json:"label,omitempty"`
}

type Attempt struct {
	ID           string  `json:"attemptID"`
	URL          string  `json:"url,omitempty"`
	Instructions string  `json:"instructions,omitempty"`
	Mode         string  `json:"mode,omitempty"`
	Created      float64 `json:"created"`
	Expires      float64 `json:"expires"`
}

type attemptWire struct {
	AttemptID    string `json:"attemptID"`
	URL          string `json:"url"`
	Instructions string `json:"instructions"`
	Mode         string `json:"mode"`
	Time         struct {
		Created finiteFloat `json:"created"`
		Expires finiteFloat `json:"expires"`
	} `json:"time"`
}

type finiteFloat float64

func (f *finiteFloat) UnmarshalJSON(data []byte) error {
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		*f = finiteFloat(value)
		return nil
	}
	var special string
	if err := json.Unmarshal(data, &special); err != nil {
		return err
	}
	*f = 0
	return nil
}

func (c *Client) ConnectKey(ctx context.Context, dir, id string, input ConnectKeyInput) error {
	return c.do(ctx, http.MethodPost, integrationPath(id)+"/connect/key"+locationQuery(dir), input, nil)
}

func (c *Client) StartOAuth(ctx context.Context, dir, id string, input ConnectMethodInput) (Attempt, error) {
	var wire attemptWire
	if err := c.do(ctx, http.MethodPost, integrationPath(id)+"/connect/oauth"+locationQuery(dir), input, &wire); err != nil {
		return Attempt{}, err
	}
	return Attempt{ID: wire.AttemptID, URL: safeExternalURL(wire.URL), Instructions: wire.Instructions, Mode: knownEnum(wire.Mode, "auto", "code"), Created: float64(wire.Time.Created), Expires: float64(wire.Time.Expires)}, nil
}

func (c *Client) StartCommand(ctx context.Context, dir, id string, input ConnectMethodInput) (Attempt, error) {
	var wire attemptWire
	input.Answer = nil
	if err := c.do(ctx, http.MethodPost, integrationPath(id)+"/connect/command"+locationQuery(dir), input, &wire); err != nil {
		return Attempt{}, err
	}
	return Attempt{ID: wire.AttemptID, Created: float64(wire.Time.Created), Expires: float64(wire.Time.Expires)}, nil
}

type AttemptStatus struct {
	Status  string  `json:"status"`
	Message string  `json:"message,omitempty"`
	Created float64 `json:"created"`
	Expires float64 `json:"expires"`
}

func (c *Client) AttemptStatus(ctx context.Context, dir, id, kind, attemptID string) (AttemptStatus, error) {
	if kind != "oauth" && kind != "command" {
		return AttemptStatus{}, errors.New("unsupported attempt kind")
	}
	var wire struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Time    struct {
			Created finiteFloat `json:"created"`
			Expires finiteFloat `json:"expires"`
		} `json:"time"`
	}
	path := integrationPath(id) + "/connect/" + kind + "/" + url.PathEscape(attemptID) + locationQuery(dir)
	if err := c.do(ctx, http.MethodGet, path, nil, &wire); err != nil {
		return AttemptStatus{}, err
	}
	status := knownEnum(wire.Status, "pending", "complete", "failed", "expired")
	message := ""
	if status == "failed" {
		message = "Connection failed. Retry the connection or check OpenCode service logs."
	}
	return AttemptStatus{Status: status, Message: message, Created: float64(wire.Time.Created), Expires: float64(wire.Time.Expires)}, nil
}

func (c *Client) CancelAttempt(ctx context.Context, dir, id, kind, attemptID string) error {
	if kind != "oauth" && kind != "command" {
		return errors.New("unsupported attempt kind")
	}
	path := integrationPath(id) + "/connect/" + kind + "/" + url.PathEscape(attemptID) + locationQuery(dir)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) CompleteOAuth(ctx context.Context, dir, id, attemptID, code string) error {
	path := integrationPath(id) + "/connect/oauth/" + url.PathEscape(attemptID) + "/complete" + locationQuery(dir)
	return c.do(ctx, http.MethodPost, path, struct {
		Code string `json:"code,omitempty"`
	}{Code: code}, nil)
}

func (c *Client) UpdateCredentialLabel(ctx context.Context, id, label string) error {
	return c.do(ctx, http.MethodPatch, "/api/credential/"+url.PathEscape(id), struct {
		Label string `json:"label"`
	}{Label: strings.TrimSpace(label)}, nil)
}

func (c *Client) ActivateCredential(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/api/credential/"+url.PathEscape(id)+"/activate", nil, nil)
}

func (c *Client) DeleteCredential(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/credential/"+url.PathEscape(id), nil, nil)
}
