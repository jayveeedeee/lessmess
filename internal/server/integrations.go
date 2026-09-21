package server

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"lessmess/internal/opencode"
)

const integrationBodyLimit = 16 << 10

func (s *Server) integrationReady(w http.ResponseWriter) bool {
	managementHeaders(w)
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OpenCode integration is unavailable."})
		return false
	}
	return true
}

func (s *Server) integrationMutationReady(w http.ResponseWriter, r *http.Request) bool {
	managementHeaders(w)
	if r.Header.Get("X-Lessmess-UI") != "1" || crossOriginRequest(r) || r.Header.Get("Origin") == "null" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "same-origin UI request required"})
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "application/json is required"})
		return false
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OpenCode integration is unavailable."})
		return false
	}
	return true
}

func integrationError(w http.ResponseWriter, err error) {
	var apiErr *opencode.APIError
	switch {
	case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "The integration, credential, or connection attempt no longer exists."})
	case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict:
		writeJSON(w, http.StatusConflict, map[string]string{"error": "The integration changed. Refresh and try again."})
	case errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusUnprocessableEntity):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "OpenCode rejected the connection request."})
	default:
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "OpenCode could not complete the integration operation."})
	}
}

func (s *Server) integrationContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 30*time.Second)
}

func (s *Server) integrationsList(w http.ResponseWriter, r *http.Request) {
	if !s.integrationReady(w) {
		return
	}
	ctx, cancel := s.integrationContext(r)
	defer cancel()
	items, err := s.oc.ListIntegrations(ctx, s.st.Dir)
	if err != nil {
		var apiErr *opencode.APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusMethodNotAllowed) {
			writeJSON(w, http.StatusOK, map[string]any{"available": false, "integrations": []opencode.IntegrationInfo{}})
			return
		}
		integrationError(w, err)
		return
	}
	if items == nil {
		items = []opencode.IntegrationInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"available": true, "integrations": items})
}

func (s *Server) integrationDetail(w http.ResponseWriter, r *http.Request) {
	if !s.integrationReady(w) {
		return
	}
	if !managementIDsReady(w, r.PathValue("integrationID")) {
		return
	}
	ctx, cancel := s.integrationContext(r)
	defer cancel()
	item, err := s.oc.GetIntegration(ctx, s.st.Dir, r.PathValue("integrationID"))
	if err != nil {
		integrationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func validAnswers(answer map[string]any) bool {
	for key, value := range answer {
		if strings.TrimSpace(key) == "" {
			return false
		}
		switch value.(type) {
		case string, float64, bool, nil, []any:
		default:
			return false
		}
		if values, ok := value.([]any); ok {
			for _, item := range values {
				if _, ok := item.(string); !ok {
					return false
				}
			}
		}
	}
	return true
}

type keyConnectRequest struct {
	Key                  string         `json:"key"`
	Label                string         `json:"label,omitempty"`
	Answer               map[string]any `json:"answer,omitempty"`
	ConfirmReplace       bool           `json:"confirmReplace,omitempty"`
	ExpectedCredentialID string         `json:"expectedCredentialID,omitempty"`
	ExpectedLabel        string         `json:"expectedLabel,omitempty"`
	Confirm              bool           `json:"confirm"`
}

func findCredential(item opencode.IntegrationInfo, id string) (opencode.IntegrationConnection, bool) {
	for _, connection := range item.Connections {
		if connection.Type == "credential" && connection.ID == id {
			return connection, true
		}
	}
	return opencode.IntegrationConnection{}, false
}

func (s *Server) checkCredential(ctx context.Context, integrationID, credentialID, expectedLabel string) (opencode.IntegrationConnection, bool) {
	item, err := s.oc.GetIntegration(ctx, s.st.Dir, integrationID)
	if err != nil {
		return opencode.IntegrationConnection{}, false
	}
	connection, ok := findCredential(item, credentialID)
	return connection, ok && connection.Label == expectedLabel
}

func (s *Server) integrationConnectKey(w http.ResponseWriter, r *http.Request) {
	if !s.integrationMutationReady(w, r) {
		return
	}
	var req keyConnectRequest
	if !decodeChatJSONLimit(w, r, &req, integrationBodyLimit) {
		return
	}
	if !managementIDsReady(w, r.PathValue("integrationID")) {
		return
	}
	if req.Key == "" || !req.Confirm || !validAnswers(req.Answer) || (req.ExpectedCredentialID != "" && !req.ConfirmReplace) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "A key and valid replacement confirmation are required."})
		return
	}
	ctx, cancel := s.integrationContext(r)
	defer cancel()
	item, err := s.oc.GetIntegration(ctx, s.st.Dir, r.PathValue("integrationID"))
	if err != nil {
		integrationError(w, err)
		return
	}
	credentials := make([]opencode.IntegrationConnection, 0)
	for _, connection := range item.Connections {
		if connection.Type == "credential" {
			credentials = append(credentials, connection)
		}
	}
	if len(credentials) > 0 && (!req.ConfirmReplace || req.ExpectedCredentialID == "") {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "The connection changed. Refresh before replacing it."})
		return
	}
	if req.ExpectedCredentialID != "" {
		matched := false
		for _, connection := range credentials {
			if connection.ID == req.ExpectedCredentialID && connection.Label == req.ExpectedLabel {
				matched = true
				break
			}
		}
		if !matched {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "The connection changed. Refresh before replacing it."})
			return
		}
	}
	err = s.oc.ConnectKey(ctx, s.st.Dir, r.PathValue("integrationID"), opencode.ConnectKeyInput{Key: req.Key, Label: req.Label, Answer: req.Answer})
	req.Key, req.Label, req.Answer = "", "", nil
	if err != nil {
		auditManagement("integration_key", "failed")
		integrationError(w, err)
		return
	}
	auditManagement("integration_key", "completed")
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

type methodConnectRequest struct {
	MethodID string         `json:"methodID"`
	Label    string         `json:"label,omitempty"`
	Answer   map[string]any `json:"answer,omitempty"`
	Confirm  bool           `json:"confirm"`
}

func (s *Server) integrationStartAttempt(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.integrationMutationReady(w, r) {
			return
		}
		var req methodConnectRequest
		if !decodeChatJSONLimit(w, r, &req, integrationBodyLimit) {
			return
		}
		if !managementIDsReady(w, r.PathValue("integrationID")) {
			return
		}
		if strings.TrimSpace(req.MethodID) == "" || !req.Confirm || !validAnswers(req.Answer) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "A supported connection method is required."})
			return
		}
		ctx, cancel := s.integrationContext(r)
		defer cancel()
		input := opencode.ConnectMethodInput{MethodID: req.MethodID, Label: req.Label, Answer: req.Answer}
		var attempt opencode.Attempt
		var err error
		if kind == "oauth" {
			attempt, err = s.oc.StartOAuth(ctx, s.st.Dir, r.PathValue("integrationID"), input)
		} else {
			attempt, err = s.oc.StartCommand(ctx, s.st.Dir, r.PathValue("integrationID"), input)
		}
		req.MethodID, req.Label, req.Answer = "", "", nil
		if err != nil {
			auditManagement("integration_attempt_start", "failed")
			integrationError(w, err)
			return
		}
		auditManagement("integration_attempt_start", "completed")
		writeJSON(w, http.StatusOK, attempt)
	}
}

func (s *Server) integrationAttemptStatus(w http.ResponseWriter, r *http.Request) {
	if !s.integrationReady(w) {
		return
	}
	if !managementIDsReady(w, r.PathValue("integrationID"), r.PathValue("attemptID")) {
		return
	}
	if r.PathValue("kind") != "oauth" && r.PathValue("kind") != "command" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid connection attempt identifier"})
		return
	}
	ctx, cancel := s.integrationContext(r)
	defer cancel()
	status, err := s.oc.AttemptStatus(ctx, s.st.Dir, r.PathValue("integrationID"), r.PathValue("kind"), r.PathValue("attemptID"))
	if err != nil {
		integrationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) integrationCancelAttempt(w http.ResponseWriter, r *http.Request) {
	if !s.integrationMutationReady(w, r) {
		return
	}
	var req struct {
		Confirm        bool   `json:"confirm"`
		ExpectedStatus string `json:"expectedStatus"`
	}
	if !decodeChatJSONLimit(w, r, &req, integrationBodyLimit) {
		return
	}
	if !req.Confirm || req.ExpectedStatus != "pending" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Cancellation confirmation is required."})
		return
	}
	if !managementIDsReady(w, r.PathValue("integrationID"), r.PathValue("attemptID")) {
		return
	}
	if r.PathValue("kind") != "oauth" && r.PathValue("kind") != "command" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid connection attempt identifier"})
		return
	}
	ctx, cancel := s.integrationContext(r)
	defer cancel()
	status, err := s.oc.AttemptStatus(ctx, s.st.Dir, r.PathValue("integrationID"), r.PathValue("kind"), r.PathValue("attemptID"))
	if err != nil {
		integrationError(w, err)
		return
	}
	if status.Status != req.ExpectedStatus {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "The connection attempt changed. Refresh and try again."})
		return
	}
	if err := s.oc.CancelAttempt(ctx, s.st.Dir, r.PathValue("integrationID"), r.PathValue("kind"), r.PathValue("attemptID")); err != nil {
		auditManagement("integration_attempt_cancel", "failed")
		integrationError(w, err)
		return
	}
	auditManagement("integration_attempt_cancel", "completed")
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) integrationCompleteOAuth(w http.ResponseWriter, r *http.Request) {
	if !s.integrationMutationReady(w, r) {
		return
	}
	var req struct {
		Code           string `json:"code"`
		Confirm        bool   `json:"confirm"`
		ExpectedStatus string `json:"expectedStatus"`
	}
	if !decodeChatJSONLimit(w, r, &req, integrationBodyLimit) {
		return
	}
	if !managementIDsReady(w, r.PathValue("integrationID"), r.PathValue("attemptID")) {
		return
	}
	if strings.TrimSpace(req.Code) == "" || !req.Confirm || req.ExpectedStatus != "pending" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "A code and completion confirmation are required."})
		return
	}
	ctx, cancel := s.integrationContext(r)
	defer cancel()
	status, err := s.oc.AttemptStatus(ctx, s.st.Dir, r.PathValue("integrationID"), "oauth", r.PathValue("attemptID"))
	if err != nil {
		auditManagement("integration_oauth_complete", "failed")
		integrationError(w, err)
		return
	}
	auditManagement("integration_oauth_complete", "completed")
	if status.Status != req.ExpectedStatus {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "The connection attempt changed. Refresh and try again."})
		return
	}
	err = s.oc.CompleteOAuth(ctx, s.st.Dir, r.PathValue("integrationID"), r.PathValue("attemptID"), req.Code)
	req.Code = ""
	if err != nil {
		integrationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "complete"})
}

type credentialMutationRequest struct {
	Label         string `json:"label,omitempty"`
	ExpectedLabel string `json:"expectedLabel,omitempty"`
	Confirm       bool   `json:"confirm,omitempty"`
}

func (s *Server) integrationCredentialAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.integrationMutationReady(w, r) {
			return
		}
		var req credentialMutationRequest
		if !decodeChatJSONLimit(w, r, &req, integrationBodyLimit) {
			return
		}
		if !managementIDsReady(w, r.PathValue("integrationID"), r.PathValue("credentialID")) || !req.Confirm {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Credential change confirmation is required."})
			return
		}
		ctx, cancel := s.integrationContext(r)
		defer cancel()
		credentialID := r.PathValue("credentialID")
		if _, ok := s.checkCredential(ctx, r.PathValue("integrationID"), credentialID, req.ExpectedLabel); !ok {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "The credential changed. Refresh and try again."})
			return
		}
		var err error
		switch action {
		case "label":
			if strings.TrimSpace(req.Label) == "" {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "A credential label is required."})
				return
			}
			err = s.oc.UpdateCredentialLabel(ctx, credentialID, req.Label)
		case "activate":
			err = s.oc.ActivateCredential(ctx, credentialID)
		case "delete":
			if !req.Confirm {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Deletion confirmation is required."})
				return
			}
			err = s.oc.DeleteCredential(ctx, credentialID)
		}
		req.Label, req.ExpectedLabel = "", ""
		if err != nil {
			auditManagement("credential_"+action, "failed")
			integrationError(w, err)
			return
		}
		auditManagement("credential_"+action, "completed")
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}
