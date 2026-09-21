package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const managementResponseLimit = 4 << 20

// ServiceInfo is the display-safe service identity. PID, URLs, and temporary
// paths returned by current services are deliberately not represented.
type ServiceInfo struct {
	Version  string `json:"version"`
	Endpoint string `json:"endpoint"`
}

// LocationInfo is the project identity for a location-scoped request.
type LocationInfo struct {
	Directory string `json:"directory"`
	Project   struct {
		ID        string `json:"id"`
		Directory string `json:"directory"`
		Canonical string `json:"canonical"`
	} `json:"project"`
}

// ProviderStatus is positive availability metadata, not a provider ping.
type ProviderStatus struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Activation string `json:"activation"`
}

// PluginStatus excludes local paths, package targets, refs, and raw failures.
type PluginStatus struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	SourceKind string `json:"sourceKind"`
	Failure    string `json:"failure,omitempty"`
}

func (p *PluginStatus) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID     string `json:"id"`
		Source struct {
			Type string `json:"type"`
		} `json:"source"`
		State struct {
			Status string `json:"status"`
		} `json:"state"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	p.ID = wire.ID
	p.SourceKind = knownEnum(wire.Source.Type, "builtin", "package", "local", "sdk")
	p.Status = knownEnum(wire.State.Status, "active", "failed")
	if p.Status == "failed" {
		p.Failure = "load_failed"
	}
	return nil
}

func knownEnum(value string, allowed ...string) string {
	for _, item := range allowed {
		if value == item {
			return value
		}
	}
	return "unknown"
}

// ManagementCapabilities is method-and-path detection for the exact read and
// future management operations lessmess understands. It never probes a
// mutating operation and does not infer behavior from a version string.
type ManagementCapabilities struct {
	ServiceInfo                       bool   `json:"serviceInfo"`
	HealthFallback                    bool   `json:"healthFallback"`
	Location                          bool   `json:"location"`
	Providers                         bool   `json:"providers"`
	Models                            bool   `json:"models"`
	DefaultModel                      bool   `json:"defaultModel"`
	Plugins                           bool   `json:"plugins"`
	PluginUpdate                      bool   `json:"pluginUpdate"`
	Integrations                      bool   `json:"integrations"`
	IntegrationKey                    bool   `json:"integrationKey"`
	IntegrationOAuth                  bool   `json:"integrationOAuth"`
	IntegrationCommand                bool   `json:"integrationCommand"`
	CredentialUpdate                  bool   `json:"credentialUpdate"`
	CredentialActivate                bool   `json:"credentialActivate"`
	CredentialDelete                  bool   `json:"credentialDelete"`
	MCP                               bool   `json:"mcp"`
	MCPResources                      bool   `json:"mcpResources"`
	MCPConnect                        bool   `json:"mcpConnect"`
	MCPDisconnect                     bool   `json:"mcpDisconnect"`
	ActivePermissions                 bool   `json:"activePermissions"`
	SavedPermissions                  bool   `json:"savedPermissions"`
	RemovePermission                  bool   `json:"removePermission"`
	ServiceRestart                    bool   `json:"serviceRestart"`
	MCPConnectPath, MCPDisconnectPath string `json:"-"`
}

func (c *Client) doDirect(ctx context.Context, path string, out any) error {
	base, password := c.connection()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("opencode", password)
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	limited := io.LimitReader(resp.Body, managementResponseLimit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return errors.New("read service response")
	}
	if len(data) > managementResponseLimit {
		return errors.New("service response too large")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return errors.New("malformed service response")
	}
	return nil
}

// ServiceIdentity prefers the published /api/info contract and falls back to
// the installed beta's /api/health only when /api/info is specifically absent.
func (c *Client) ServiceIdentity(ctx context.Context) (ServiceInfo, error) {
	var info struct {
		Version string `json:"version"`
	}
	if err := c.doDirect(ctx, "/api/info", &info); err == nil {
		if strings.TrimSpace(info.Version) == "" {
			return ServiceInfo{}, errors.New("malformed service response")
		}
		return ServiceInfo{Version: info.Version, Endpoint: "info"}, nil
	} else {
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			return ServiceInfo{}, err
		}
	}
	var health struct {
		Version string `json:"version"`
		Healthy *bool  `json:"healthy"`
	}
	if err := c.doDirect(ctx, "/api/health", &health); err != nil {
		return ServiceInfo{}, err
	}
	if health.Healthy != nil && !*health.Healthy {
		return ServiceInfo{}, errors.New("service not ready")
	}
	if strings.TrimSpace(health.Version) == "" {
		return ServiceInfo{}, errors.New("malformed service response")
	}
	return ServiceInfo{Version: health.Version, Endpoint: "health_fallback"}, nil
}

func (c *Client) ProjectLocation(ctx context.Context, dir string) (LocationInfo, error) {
	var location LocationInfo
	if err := c.doDirect(ctx, "/api/location"+locationQuery(dir), &location); err != nil {
		return LocationInfo{}, err
	}
	if location.Directory == "" || location.Project.ID == "" {
		return LocationInfo{}, errors.New("malformed service response")
	}
	return location, nil
}

func (c *Client) ListProvidersFor(ctx context.Context, dir string) ([]ProviderStatus, error) {
	var providers []ProviderStatus
	if err := c.do(ctx, http.MethodGet, "/api/provider"+locationQuery(dir), nil, &providers); err != nil {
		return nil, err
	}
	for i := range providers {
		providers[i].Activation = knownEnum(providers[i].Activation, "auto", "enabled", "disabled")
	}
	return providers, nil
}

func (c *Client) ListPluginsFor(ctx context.Context, dir string) ([]PluginStatus, error) {
	var plugins []PluginStatus
	if err := c.do(ctx, http.MethodGet, "/api/plugin"+locationQuery(dir), nil, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

func (c *Client) ManagementCapabilities(ctx context.Context) (ManagementCapabilities, error) {
	spec, err := c.openAPI(ctx)
	if err != nil {
		return ManagementCapabilities{}, err
	}
	has := func(method, path string) bool { return hasOperation(spec, method, path) }
	cap := ManagementCapabilities{
		ServiceInfo: has(http.MethodGet, "/api/info"), HealthFallback: has(http.MethodGet, "/api/health"),
		Location: has(http.MethodGet, "/api/location"), Providers: has(http.MethodGet, "/api/provider"),
		Models: has(http.MethodGet, "/api/model"), DefaultModel: has(http.MethodGet, "/api/model/default"),
		Plugins: has(http.MethodGet, "/api/plugin"), PluginUpdate: has(http.MethodPost, "/api/plugin/update"),
		Integrations: has(http.MethodGet, "/api/integration"), MCP: has(http.MethodGet, "/api/mcp"),
		IntegrationKey:     has(http.MethodPost, "/api/integration/{integrationID}/connect/key"),
		IntegrationOAuth:   has(http.MethodPost, "/api/integration/{integrationID}/connect/oauth") && has(http.MethodGet, "/api/integration/{integrationID}/connect/oauth/{attemptID}") && has(http.MethodDelete, "/api/integration/{integrationID}/connect/oauth/{attemptID}") && has(http.MethodPost, "/api/integration/{integrationID}/connect/oauth/{attemptID}/complete"),
		IntegrationCommand: has(http.MethodPost, "/api/integration/{integrationID}/connect/command") && has(http.MethodGet, "/api/integration/{integrationID}/connect/command/{attemptID}") && has(http.MethodDelete, "/api/integration/{integrationID}/connect/command/{attemptID}"),
		CredentialUpdate:   has(http.MethodPatch, "/api/credential/{credentialID}"), CredentialActivate: has(http.MethodPost, "/api/credential/{credentialID}/activate"), CredentialDelete: has(http.MethodDelete, "/api/credential/{credentialID}"),
		MCPResources:      has(http.MethodGet, "/api/mcp/resource"),
		ActivePermissions: has(http.MethodGet, "/api/permission/request"),
		SavedPermissions:  has(http.MethodGet, "/api/permission/saved"), RemovePermission: has(http.MethodDelete, "/api/permission/saved/{id}"),
	}
	for _, candidate := range []struct {
		path    *string
		enabled *bool
		value   string
	}{
		{&cap.MCPConnectPath, &cap.MCPConnect, "/api/experimental/mcp/{server}/connect"},
		{&cap.MCPConnectPath, &cap.MCPConnect, "/api/mcp/{server}/connect"},
		{&cap.MCPDisconnectPath, &cap.MCPDisconnect, "/api/experimental/mcp/{server}/disconnect"},
		{&cap.MCPDisconnectPath, &cap.MCPDisconnect, "/api/mcp/{server}/disconnect"},
	} {
		if !*candidate.enabled && has(http.MethodPost, candidate.value) {
			*candidate.enabled, *candidate.path = true, candidate.value
		}
	}
	return cap, nil
}
