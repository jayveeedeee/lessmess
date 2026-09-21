package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"lessmess/internal/opencode"
)

type mcpServerView struct {
	Name              string `json:"name"`
	Status            string `json:"status"`
	Failure           string `json:"failure,omitempty"`
	AuthIntegrationID string `json:"authIntegrationID,omitempty"`
}

type mcpOverviewResponse struct {
	Available           bool                           `json:"available"`
	ResourcesAvailable  bool                           `json:"resourcesAvailable"`
	ConnectAvailable    bool                           `json:"connectAvailable"`
	DisconnectAvailable bool                           `json:"disconnectAvailable"`
	Servers             []mcpServerView                `json:"servers"`
	Resources           []opencode.MCPResource         `json:"resources"`
	Templates           []opencode.MCPResourceTemplate `json:"templates"`
	RuntimeNotice       string                         `json:"runtimeNotice"`
}

func managementContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 15*time.Second)
}

func managementOperationError(w http.ResponseWriter, err error, noun string) {
	if errors.Is(err, opencode.ErrCapabilityUnavailable) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": noun + " is not supported by this OpenCode service."})
		return
	}
	var apiErr *opencode.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": noun + " no longer exists. Refresh and try again."})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "OpenCode could not complete the " + noun + " operation."})
}

func (s *Server) mcpOverview(w http.ResponseWriter, r *http.Request) {
	if !s.integrationReady(w) {
		return
	}
	ctx, cancel := managementContext(r)
	defer cancel()
	cap, err := s.oc.ManagementCapabilities(ctx)
	if err != nil {
		managementOperationError(w, err, "MCP overview")
		return
	}
	view := mcpOverviewResponse{
		Available: cap.MCP, ResourcesAvailable: cap.MCPResources,
		ConnectAvailable: cap.MCPConnect, DisconnectAvailable: cap.MCPDisconnect,
		Servers: []mcpServerView{}, Resources: []opencode.MCPResource{}, Templates: []opencode.MCPResourceTemplate{},
		RuntimeNotice: "Connect and disconnect change OpenCode runtime state and may not survive a service restart.",
	}
	if !cap.MCP {
		writeJSON(w, http.StatusOK, view)
		return
	}
	servers, err := s.oc.ListMCPFor(ctx, cap, s.st.Dir)
	if err != nil {
		managementOperationError(w, err, "MCP overview")
		return
	}
	integrations := map[string]bool{}
	for _, server := range servers {
		if server.Status == "needs_auth" && server.IntegrationID != "" {
			integrations[server.IntegrationID] = false
		}
	}
	if len(integrations) > 0 {
		if list, listErr := s.oc.ListIntegrations(ctx, s.st.Dir); listErr == nil {
			for _, item := range list {
				if _, ok := integrations[item.ID]; ok {
					integrations[item.ID] = true
				}
			}
		}
	}
	for _, server := range servers {
		item := mcpServerView{Name: server.Name, Status: server.Status, Failure: server.Failure}
		if integrations[server.IntegrationID] {
			item.AuthIntegrationID = server.IntegrationID
		}
		view.Servers = append(view.Servers, item)
	}
	if cap.MCPResources {
		catalog, resourceErr := s.oc.ListMCPResourcesFor(ctx, cap, s.st.Dir)
		if resourceErr == nil {
			view.Resources, view.Templates = catalog.Resources, catalog.Templates
		} else {
			view.ResourcesAvailable = false
		}
	}
	writeJSON(w, http.StatusOK, view)
}

type mcpMutationRequest struct {
	ExpectedStatus string `json:"expectedStatus"`
	Confirm        bool   `json:"confirm,omitempty"`
}

func (s *Server) mcpConnectionAction(connect bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.integrationMutationReady(w, r) {
			return
		}
		var req mcpMutationRequest
		if !decodeChatJSONLimit(w, r, &req, integrationBodyLimit) {
			return
		}
		if !managementIDsReady(w, r.PathValue("server")) {
			return
		}
		if req.ExpectedStatus == "" || (!connect && !req.Confirm) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Current status and disconnect confirmation are required."})
			return
		}
		if (connect && req.ExpectedStatus != "disabled" && req.ExpectedStatus != "failed") || (!connect && req.ExpectedStatus != "connected") {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "That MCP state does not support this runtime action."})
			return
		}
		ctx, cancel := managementContext(r)
		defer cancel()
		cap, err := s.oc.ManagementCapabilities(ctx)
		if err != nil {
			managementOperationError(w, err, "MCP connection")
			return
		}
		servers, err := s.oc.ListMCPFor(ctx, cap, s.st.Dir)
		if err != nil {
			managementOperationError(w, err, "MCP connection")
			return
		}
		name := r.PathValue("server")
		matched := false
		for _, server := range servers {
			if server.Name == name && server.Status == req.ExpectedStatus {
				matched = true
				break
			}
		}
		if !matched {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "The MCP server status changed. Refresh and try again."})
			return
		}
		if err := s.oc.SetMCPConnected(ctx, cap, s.st.Dir, name, connect); err != nil {
			auditManagement("mcp_connection", "failed")
			managementOperationError(w, err, "MCP connection")
			return
		}
		auditManagement("mcp_connection", "completed")
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

type activePermissionView struct {
	ID            string `json:"id"`
	SessionID     string `json:"sessionID"`
	Action        string `json:"action"`
	ResourceCount int    `json:"resourceCount"`
	Chat          bool   `json:"chat"`
	SessionTitle  string `json:"sessionTitle,omitempty"`
}

type permissionOverviewResponse struct {
	ActiveAvailable bool                       `json:"activeAvailable"`
	SavedAvailable  bool                       `json:"savedAvailable"`
	RemoveAvailable bool                       `json:"removeAvailable"`
	Active          []activePermissionView     `json:"active"`
	Saved           []opencode.SavedPermission `json:"saved"`
}

func (s *Server) permissionOverview(w http.ResponseWriter, r *http.Request) {
	if !s.integrationReady(w) {
		return
	}
	ctx, cancel := managementContext(r)
	defer cancel()
	cap, err := s.oc.ManagementCapabilities(ctx)
	if err != nil {
		managementOperationError(w, err, "permission overview")
		return
	}
	view := permissionOverviewResponse{
		ActiveAvailable: cap.ActivePermissions, SavedAvailable: cap.SavedPermissions,
		RemoveAvailable: cap.RemovePermission, Active: []activePermissionView{}, Saved: []opencode.SavedPermission{},
	}
	if cap.ActivePermissions {
		active, listErr := s.oc.ListActivePermissionsFor(ctx, cap, s.st.Dir)
		if listErr != nil {
			managementOperationError(w, listErr, "active permission overview")
			return
		}
		for _, item := range active {
			pv := activePermissionView{ID: item.ID, SessionID: item.SessionID, Action: item.Action, ResourceCount: item.ResourceCount}
			if _, entry, ok := s.sessions.entry(item.SessionID); ok {
				pv.Chat, pv.SessionTitle = true, entry.Title
			}
			view.Active = append(view.Active, pv)
		}
	}
	if cap.SavedPermissions {
		location, locationErr := s.oc.ProjectLocation(ctx, s.st.Dir)
		if locationErr != nil {
			managementOperationError(w, locationErr, "project location")
			return
		}
		view.Saved, err = s.oc.ListSavedPermissions(ctx, cap, location.Project.ID)
		if err != nil {
			managementOperationError(w, err, "saved permission overview")
			return
		}
		if view.Saved == nil {
			view.Saved = []opencode.SavedPermission{}
		}
	}
	writeJSON(w, http.StatusOK, view)
}

type savedPermissionRemoveRequest struct {
	Confirm          bool   `json:"confirm"`
	ExpectedProject  string `json:"expectedProject"`
	ExpectedAction   string `json:"expectedAction"`
	ExpectedResource string `json:"expectedResource"`
}

func (s *Server) savedPermissionRemove(w http.ResponseWriter, r *http.Request) {
	if !s.integrationMutationReady(w, r) {
		return
	}
	var req savedPermissionRemoveRequest
	if !decodeChatJSONLimit(w, r, &req, integrationBodyLimit) {
		return
	}
	if !managementIDsReady(w, r.PathValue("ruleID")) {
		return
	}
	if !req.Confirm || req.ExpectedProject == "" || req.ExpectedAction == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Rule identity and removal confirmation are required."})
		return
	}
	ctx, cancel := managementContext(r)
	defer cancel()
	cap, err := s.oc.ManagementCapabilities(ctx)
	if err != nil {
		managementOperationError(w, err, "saved permission removal")
		return
	}
	location, err := s.oc.ProjectLocation(ctx, s.st.Dir)
	if err != nil {
		managementOperationError(w, err, "project location")
		return
	}
	if req.ExpectedProject != location.Project.ID {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "The served project changed. Refresh and try again."})
		return
	}
	rules, err := s.oc.ListSavedPermissions(ctx, cap, location.Project.ID)
	if err != nil {
		managementOperationError(w, err, "saved permission removal")
		return
	}
	matched := false
	for _, rule := range rules {
		if rule.ID == r.PathValue("ruleID") && rule.ProjectID == req.ExpectedProject && rule.Action == req.ExpectedAction && rule.Resource == req.ExpectedResource {
			matched = true
			break
		}
	}
	if !matched {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "The saved rule changed. Refresh before removing it."})
		return
	}
	if err := s.oc.RemoveSavedPermission(ctx, cap, r.PathValue("ruleID")); err != nil {
		auditManagement("saved_permission_remove", "failed")
		managementOperationError(w, err, "saved permission removal")
		return
	}
	auditManagement("saved_permission_remove", "completed")
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
