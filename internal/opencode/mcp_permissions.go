package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// MCPServer is the display-safe subset of an MCP server. Raw connection
// failures are reduced to stable categories because they can contain command
// lines, headers, URLs, and other configuration details.
type MCPServer struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	Failure       string `json:"failure,omitempty"`
	IntegrationID string `json:"integrationID,omitempty"`
}

func (m *MCPServer) UnmarshalJSON(data []byte) error {
	var wire struct {
		Name   string `json:"name"`
		Status struct {
			Status string `json:"status"`
		} `json:"status"`
		IntegrationID string `json:"integrationID"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	m.Name, m.IntegrationID = wire.Name, wire.IntegrationID
	m.Status = knownEnum(wire.Status.Status, "connected", "pending", "disabled", "failed", "needs_auth")
	switch m.Status {
	case "failed":
		m.Failure = "connection_failed"
	case "needs_auth":
		m.Failure = "authentication_required"
	}
	return nil
}

type MCPResource struct {
	Server   string `json:"server"`
	Name     string `json:"name"`
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
}

type MCPResourceTemplate struct {
	Server      string `json:"server"`
	Name        string `json:"name"`
	URITemplate string `json:"uriTemplate"`
	MIMEType    string `json:"mimeType,omitempty"`
}

func (r *MCPResource) UnmarshalJSON(data []byte) error {
	type resource MCPResource
	var wire resource
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*r = MCPResource(wire)
	r.URI = redactResourceURI(r.URI)
	return nil
}

// redactResourceURI retains only a useful scheme/host identity. Userinfo,
// query values, fragments, opaque values, and resource paths stay server-side.
func redactResourceURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return "<redacted>"
	}
	u.User, u.RawQuery, u.Fragment, u.RawFragment = nil, "", "", ""
	if u.Opaque != "" {
		u.Opaque = "<redacted>"
		return u.String()
	}
	if u.Path != "" {
		u.Path, u.RawPath = "/<redacted>", ""
	}
	return u.String()
}

type MCPResourceCatalog struct {
	Resources []MCPResource         `json:"resources"`
	Templates []MCPResourceTemplate `json:"templates"`
}

func (c *MCPResourceCatalog) UnmarshalJSON(data []byte) error {
	type catalog MCPResourceCatalog
	var wire catalog
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*c = MCPResourceCatalog(wire)
	for i := range c.Templates {
		c.Templates[i].URITemplate = redactResourceURI(c.Templates[i].URITemplate)
	}
	return nil
}

type ActivePermission struct {
	ID            string `json:"id"`
	SessionID     string `json:"sessionID"`
	Action        string `json:"action"`
	ResourceCount int    `json:"resourceCount"`
}

func (p *ActivePermission) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID        string   `json:"id"`
		SessionID string   `json:"sessionID"`
		Action    string   `json:"action"`
		Resources []string `json:"resources"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	p.ID, p.SessionID, p.Action, p.ResourceCount = wire.ID, wire.SessionID, wire.Action, len(wire.Resources)
	return nil
}

// SavedPermission is an allow rule persisted by OpenCode for one project.
// OpenCode exposes no create/edit operation for this store.
type SavedPermission struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
}

func (c *Client) ListMCPFor(ctx context.Context, cap ManagementCapabilities, dir string) ([]MCPServer, error) {
	if err := unavailable(cap.MCP, "MCP server list"); err != nil {
		return nil, err
	}
	var servers []MCPServer
	if err := c.do(ctx, http.MethodGet, "/api/mcp"+locationQuery(dir), nil, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func (c *Client) ListMCPResourcesFor(ctx context.Context, cap ManagementCapabilities, dir string) (MCPResourceCatalog, error) {
	if err := unavailable(cap.MCPResources, "MCP resource list"); err != nil {
		return MCPResourceCatalog{}, err
	}
	var catalog MCPResourceCatalog
	if err := c.do(ctx, http.MethodGet, "/api/mcp/resource"+locationQuery(dir), nil, &catalog); err != nil {
		return MCPResourceCatalog{}, err
	}
	return catalog, nil
}

func (c *Client) SetMCPConnected(ctx context.Context, cap ManagementCapabilities, dir, server string, connect bool) error {
	path, ok := cap.MCPDisconnectPath, cap.MCPDisconnect
	if connect {
		path, ok = cap.MCPConnectPath, cap.MCPConnect
	}
	if err := unavailable(ok, "MCP runtime connection control"); err != nil {
		return err
	}
	path = strings.Replace(path, "{server}", url.PathEscape(server), 1)
	return c.do(ctx, http.MethodPost, path+locationQuery(dir), nil, nil)
}

func (c *Client) ListActivePermissionsFor(ctx context.Context, cap ManagementCapabilities, dir string) ([]ActivePermission, error) {
	if err := unavailable(cap.ActivePermissions, "active permission list"); err != nil {
		return nil, err
	}
	var requests []ActivePermission
	if err := c.do(ctx, http.MethodGet, "/api/permission/request"+locationQuery(dir), nil, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

func (c *Client) ListSavedPermissions(ctx context.Context, cap ManagementCapabilities, projectID string) ([]SavedPermission, error) {
	if err := unavailable(cap.SavedPermissions, "saved permission list"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project ID is required")
	}
	var rules []SavedPermission
	if err := c.do(ctx, http.MethodGet, "/api/permission/saved?projectID="+url.QueryEscape(projectID), nil, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *Client) RemoveSavedPermission(ctx context.Context, cap ManagementCapabilities, id string) error {
	if err := unavailable(cap.RemovePermission, "saved permission removal"); err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, "/api/permission/saved/"+url.PathEscape(id), nil, nil)
}
