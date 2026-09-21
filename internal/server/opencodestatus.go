package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"lessmess/internal/opencode"
)

type opencodeFinding struct {
	State   string `json:"state"`
	Failure string `json:"failure,omitempty"`
	Message string `json:"message,omitempty"`
}

type opencodeProjectView struct {
	ID                string `json:"id,omitempty"`
	Name              string `json:"name"`
	MatchesRepository bool   `json:"matchesRepository"`
}

type opencodeStatusResponse struct {
	State        string                          `json:"state"`
	Service      opencodeFinding                 `json:"service"`
	Version      string                          `json:"version,omitempty"`
	IdentityVia  string                          `json:"identityVia,omitempty"`
	Project      opencodeFinding                 `json:"project"`
	Location     *opencodeProjectView            `json:"location,omitempty"`
	Providers    opencodeFinding                 `json:"providers"`
	ProviderList []opencode.ProviderStatus       `json:"providerList"`
	Models       opencodeFinding                 `json:"models"`
	ModelList    []opencodeModelStatus           `json:"modelList"`
	DefaultModel *opencodeModelStatus            `json:"defaultModel,omitempty"`
	Plugins      opencodeFinding                 `json:"plugins"`
	PluginList   []opencode.PluginStatus         `json:"pluginList"`
	Capabilities opencode.ManagementCapabilities `json:"capabilities"`
	Capability   opencodeFinding                 `json:"capability"`
}

type opencodeModelStatus struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Status     string `json:"status"`
}

func managementHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func auditManagement(operation, outcome string) {
	slog.Info("OpenCode management operation", "operation", operation, "outcome", outcome)
}

func normalizedOCFailure(err error) opencodeFinding {
	var apiErr *opencode.APIError
	var netErr net.Error
	switch {
	case err == nil:
		return opencodeFinding{State: "available"}
	case errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusMethodNotAllowed):
		return opencodeFinding{State: "unsupported", Failure: "capability_unsupported", Message: "This OpenCode service does not provide this status endpoint."}
	case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized:
		return opencodeFinding{State: "degraded", Failure: "service_unauthorized", Message: "The saved OpenCode service credentials are stale."}
	case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusServiceUnavailable:
		return opencodeFinding{State: "degraded", Failure: "upstream_unavailable", Message: "OpenCode is reachable, but this subsystem is unavailable."}
	case errors.As(err, &netErr), errors.Is(err, context.DeadlineExceeded):
		return opencodeFinding{State: "degraded", Failure: "service_unreachable", Message: "The configured OpenCode service cannot be reached."}
	default:
		return opencodeFinding{State: "degraded", Failure: "incompatible_response", Message: "OpenCode returned an incompatible status response."}
	}
}

func safeStatusEnum(value string) string {
	switch value {
	case "alpha", "beta", "deprecated", "active":
		return value
	default:
		return "unknown"
	}
}

func (s *Server) collectOpencodeStatus(ctx context.Context) opencodeStatusResponse {
	resp := opencodeStatusResponse{
		State: "degraded", Service: opencodeFinding{State: "degraded", Failure: "service_unavailable", Message: "OpenCode was unavailable when lessmess started."},
		Project: opencodeFinding{State: "unsupported"}, Providers: opencodeFinding{State: "unsupported"},
		Models: opencodeFinding{State: "unsupported"}, Plugins: opencodeFinding{State: "unsupported"},
		Capability:   opencodeFinding{State: "unsupported", Failure: "capability_unavailable", Message: "Operation capabilities could not be read."},
		ProviderList: []opencode.ProviderStatus{}, ModelList: []opencodeModelStatus{}, PluginList: []opencode.PluginStatus{},
	}
	if s.oc == nil {
		return resp
	}
	info, err := s.oc.ServiceIdentity(ctx)
	resp.Service = normalizedOCFailure(err)
	if err != nil {
		return resp
	}
	resp.Service.State = "healthy"
	resp.Version, resp.IdentityVia = info.Version, info.Endpoint

	type result struct {
		kind string
		data any
		err  error
	}
	results := make(chan result, 6)
	go func() { v, e := s.oc.ProjectLocation(ctx, s.st.Dir); results <- result{"project", v, e} }()
	go func() { v, e := s.oc.ListProvidersFor(ctx, s.st.Dir); results <- result{"providers", v, e} }()
	go func() { v, e := s.oc.ListModelsFor(ctx, s.st.Dir); results <- result{"models", v, e} }()
	go func() { v, e := s.oc.DefaultModel(ctx); results <- result{"default", v, e} }()
	go func() { v, e := s.oc.ListPluginsFor(ctx, s.st.Dir); results <- result{"plugins", v, e} }()
	go func() { v, e := s.oc.ManagementCapabilities(ctx); results <- result{"capabilities", v, e} }()
	var defaultErr error
	for range 6 {
		r := <-results
		switch r.kind {
		case "project":
			resp.Project = normalizedOCFailure(r.err)
			if r.err == nil {
				v := r.data.(opencode.LocationInfo)
				resp.Location = &opencodeProjectView{ID: v.Project.ID, Name: filepath.Base(s.st.Dir), MatchesRepository: sameDirectory(v.Directory, s.st.Dir)}
			}
		case "providers":
			resp.Providers = normalizedOCFailure(r.err)
			if r.err == nil {
				resp.ProviderList = r.data.([]opencode.ProviderStatus)
			}
		case "models":
			resp.Models = normalizedOCFailure(r.err)
			if r.err == nil {
				for _, m := range r.data.([]opencode.ModelInfo) {
					resp.ModelList = append(resp.ModelList, opencodeModelStatus{ID: m.ID, ProviderID: m.ProviderID, Name: m.Name, Enabled: m.IsEnabled(), Status: safeStatusEnum(m.Status)})
				}
			}
		case "default":
			defaultErr = r.err
			if r.err == nil && r.data.(*opencode.ModelInfo) != nil {
				m := r.data.(*opencode.ModelInfo)
				resp.DefaultModel = &opencodeModelStatus{ID: m.ID, ProviderID: m.ProviderID, Name: m.Name, Enabled: m.IsEnabled(), Status: safeStatusEnum(m.Status)}
			}
		case "plugins":
			resp.Plugins = normalizedOCFailure(r.err)
			if r.err == nil {
				resp.PluginList = r.data.([]opencode.PluginStatus)
			}
		case "capabilities":
			resp.Capability = normalizedOCFailure(r.err)
			if r.err == nil {
				resp.Capabilities = r.data.(opencode.ManagementCapabilities)
			}
		}
	}
	if defaultErr != nil && resp.Models.State == "available" {
		resp.Models = normalizedOCFailure(defaultErr)
		resp.Models.Message = "Models are available, but the default model could not be read."
	}
	resp.State = "healthy"
	for _, finding := range []opencodeFinding{resp.Project, resp.Providers, resp.Models, resp.Plugins, resp.Capability} {
		if finding.State != "available" {
			resp.State = "partial"
			break
		}
	}
	return resp
}

func sameDirectory(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func (s *Server) opencodeStatusAPI(w http.ResponseWriter, r *http.Request) {
	managementHeaders(w)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, s.collectOpencodeStatus(ctx))
}

func (s *Server) opencodeRediscover(w http.ResponseWriter, r *http.Request) {
	managementHeaders(w)
	if r.Header.Get("X-Lessmess-UI") != "1" || crossOriginRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "same-origin UI request required"})
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OpenCode was unavailable at startup; restart lessmess after starting the service."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.oc.Rediscover(ctx); err != nil {
		auditManagement("rediscover", "failed")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": normalizedOCFailure(err).Message})
		return
	}
	auditManagement("rediscover", "completed")
	writeJSON(w, http.StatusOK, s.collectOpencodeStatus(ctx))
}

func crossOriginRequest(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.Host != r.Host || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return true
	}
	wantScheme := "http"
	if r.TLS != nil {
		wantScheme = "https"
	}
	return u.Scheme != wantScheme
}

func validManagementID(value string) bool {
	if value == "" || len(value) > 256 || value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func managementIDsReady(w http.ResponseWriter, values ...string) bool {
	for _, value := range values {
		if !validManagementID(value) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid management resource identifier"})
			return false
		}
	}
	return true
}

func (s *Server) opencodeStatusPage(w http.ResponseWriter, r *http.Request) {
	managementHeaders(w)
	s.rend.render(w, s.rend.opencode, "layout", pageData{Title: "OpenCode status", Page: "opencode"})
}
