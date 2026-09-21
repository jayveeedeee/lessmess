package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"lessmess/internal/opencode"
)

type lifecycleConfirmation struct {
	SessionID string `json:"sessionID"`
	Updated   int64  `json:"updated"`
	MessageID string `json:"messageID,omitempty"`
}

func (s *Server) lifecycleReady(w http.ResponseWriter, sessionID string) bool {
	return s.chatMutationReady(w, sessionID, "", "")
}

func (s *Server) lifecycleCapabilities(w http.ResponseWriter, r *http.Request) (opencode.LifecycleCapabilities, bool) {
	cap, err := s.oc.LifecycleCapabilities(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OpenCode lifecycle capabilities are unavailable"})
		return cap, false
	}
	return cap, true
}

func (s *Server) confirmedIdle(w http.ResponseWriter, r *http.Request, sessionID string, confirmation lifecycleConfirmation) (*opencode.Session, bool) {
	if confirmation.SessionID == "" || confirmation.Updated == 0 {
		writeJSON(w, http.StatusPreconditionRequired, map[string]string{"error": "confirmation with sessionID and updated is required"})
		return nil, false
	}
	if confirmation.SessionID != sessionID {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "confirmation does not match this session"})
		return nil, false
	}
	session, err := s.oc.GetSession(r.Context(), sessionID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return nil, false
	}
	if session.Time.Updated != confirmation.Updated {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "session changed after confirmation; refresh and retry"})
		return nil, false
	}
	active, err := s.oc.ListActiveSessions(r.Context())
	if err != nil {
		writeChatUpstreamError(w, err)
		return nil, false
	}
	if active[sessionID].Type == "running" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "session is busy"})
		return nil, false
	}
	return session, true
}

func writeLifecycleError(w http.ResponseWriter, err error) {
	if errors.Is(err, opencode.ErrCapabilityUnavailable) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "This operation is unavailable in the connected OpenCode service"})
		return
	}
	writeChatUpstreamError(w, err)
}

func (s *Server) sessionLifecycleInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	session, err := s.oc.GetSession(r.Context(), id)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	active, err := s.oc.ListActiveSessions(r.Context())
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	var mapping any
	if owner, entry, found := s.sessions.entry(id); found {
		mapping = mappedSessionOwner{Session: id, Change: owner, Task: entry.Task, Title: entry.Title}
		if owner == unassignedKey {
			mapping = mappedSessionOwner{Session: id, Task: entry.Task, Title: entry.Title}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session": map[string]any{"id": session.ID, "title": session.Title, "parentID": session.ParentID, "updated": session.Time.Updated, "revert": session.Revert},
		"busy":    active[id].Type == "running", "capabilities": cap, "mapping": mapping,
	})
}

type sessionNavigationItem struct {
	Session      string `json:"session"`
	Title        string `json:"title"`
	ParentID     string `json:"parentID,omitempty"`
	Change       string `json:"change,omitempty"`
	Task         string `json:"task,omitempty"`
	Depth        int    `json:"depth,omitempty"`
	PendingInput int    `json:"pendingInput"`
	Busy         bool   `json:"busy"`
	Live         bool   `json:"live"`
}

func (s *Server) navigationItem(ctx context.Context, session opencode.Session, depth int, active map[string]opencode.SessionActive) sessionNavigationItem {
	item := sessionNavigationItem{Session: session.ID, Title: session.Title, ParentID: session.ParentID, Depth: depth, Busy: active[session.ID].Type == "running", Live: true}
	if change, entry, ok := s.sessions.entry(session.ID); ok {
		if change != unassignedKey {
			item.Change = change
		}
		item.Task = entry.Task
	}
	if requests, err := s.oc.ListPermissions(ctx, session.ID); err == nil {
		for _, request := range requests {
			if request.SessionID == session.ID {
				item.PendingInput++
			}
		}
	}
	if forms, err := s.oc.ListForms(ctx, session.ID); err == nil {
		for _, form := range forms {
			if form.SessionID == session.ID {
				item.PendingInput++
			}
		}
	}
	return item
}

// sessionNavigation returns one authoritative family view for Chat. Parent
// breadcrumbs are fetched from Session.Info.parentID and descendants from the
// paginated direct-child route; mapping annotations only add ownership/task
// metadata and never define hierarchy.
func (s *Server) sessionNavigation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unavailable"})
		return
	}
	if !onlyChatQueries(w, r) {
		return
	}
	current, err := s.oc.GetSession(r.Context(), id)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	if change, _, ok := s.sessions.entry(id); ok && change != unassignedKey {
		if c, loadErr := s.st.Change(change); loadErr == nil {
			s.reconcileTaskSessions(r, c)
		}
	}

	seen := map[string]bool{id: true}
	var ancestors []opencode.Session
	parentID := current.ParentID
	for parentID != "" && !seen[parentID] && r.Context().Err() == nil {
		seen[parentID] = true
		parent, getErr := s.oc.GetSession(r.Context(), parentID)
		if getErr != nil {
			break
		}
		ancestors = append(ancestors, *parent)
		parentID = parent.ParentID
	}
	for left, right := 0, len(ancestors)-1; left < right; left, right = left+1, right-1 {
		ancestors[left], ancestors[right] = ancestors[right], ancestors[left]
	}
	descendants := s.sessionDescendants(r.Context(), []string{id})
	active, err := s.oc.ListActiveSessions(r.Context())
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	ancestorItems := make([]sessionNavigationItem, 0, len(ancestors))
	for _, session := range ancestors {
		ancestorItems = append(ancestorItems, s.navigationItem(r.Context(), session, 0, active))
	}
	descendantItems := make([]sessionNavigationItem, 0, len(descendants))
	for _, descendant := range descendants {
		descendantItems = append(descendantItems, s.navigationItem(r.Context(), descendant.Session, descendant.Depth, active))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current":     s.navigationItem(r.Context(), *current, 0, active),
		"ancestors":   ancestorItems,
		"descendants": descendantItems,
	})
}

func (s *Server) sessionFork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unavailable"})
		return
	}
	var req struct {
		Before string `json:"before"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	if !validChatID(req.Before, "msg_") {
		writeJSON(w, 400, map[string]string{"error": "valid msg_ before is required"})
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	child, err := s.oc.ForkSession(r.Context(), cap, id, req.Before)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	if change, parent, found := s.sessions.entry(id); found {
		entry := SessionEntry{Session: child.ID, Title: child.Title, Created: time.UnixMilli(child.Time.Created).UTC().Format(time.RFC3339), Task: parent.Task, Parent: id}
		var mapErr error
		if change == unassignedKey {
			mapErr = s.sessions.addUnassigned(entry)
		} else {
			mapErr = s.sessions.add(change, entry)
		}
		if mapErr != nil {
			_ = s.oc.DeleteSession(r.Context(), child.ID)
			writeJSON(w, 500, map[string]string{"error": "fork created but session mapping could not be saved"})
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"session": map[string]any{"id": child.ID, "title": child.Title, "parentID": child.ParentID}})
}

func (s *Server) sessionRevertPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	session, err := s.oc.GetSession(r.Context(), id)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	if session.Revert == nil {
		writeJSON(w, http.StatusOK, map[string]any{"staged": false, "files": []opencode.FileDiff{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"staged": true, "messageID": session.Revert.MessageID, "files": session.Revert.Files, "updated": session.Time.Updated})
}

func (s *Server) sessionRevertStage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	var req struct {
		MessageID    string                `json:"messageID"`
		Files        bool                  `json:"files"`
		Confirmation lifecycleConfirmation `json:"confirmation"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	if !validChatID(req.MessageID, "msg_") {
		writeJSON(w, 400, map[string]string{"error": "valid msg_ messageID is required"})
		return
	}
	if _, ok := s.confirmedIdle(w, r, id, req.Confirmation); !ok {
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	if err := s.oc.StageRevert(r.Context(), cap, id, req.MessageID, req.Files); err != nil {
		writeLifecycleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) sessionRevertCommit(w http.ResponseWriter, r *http.Request) {
	s.revertFinal(w, r, true)
}
func (s *Server) sessionRevertClear(w http.ResponseWriter, r *http.Request) {
	s.revertFinal(w, r, false)
}
func (s *Server) revertFinal(w http.ResponseWriter, r *http.Request, commit bool) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	var req struct {
		Confirmation lifecycleConfirmation `json:"confirmation"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	session, ok := s.confirmedIdle(w, r, id, req.Confirmation)
	if !ok {
		return
	}
	if session.Revert == nil {
		writeJSON(w, 409, map[string]string{"error": "no revert is staged"})
		return
	}
	if req.Confirmation.MessageID == "" || req.Confirmation.MessageID != session.Revert.MessageID {
		writeJSON(w, 409, map[string]string{"error": "confirmation does not match the staged revert"})
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	var err error
	if commit {
		err = s.oc.CommitRevert(r.Context(), cap, id)
	} else {
		err = s.oc.ClearRevert(r.Context(), cap, id)
	}
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validDelivery(value opencode.Delivery) bool {
	return value == opencode.DeliveryQueue || value == opencode.DeliverySteer
}
func (s *Server) sessionCompact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	var req struct {
		MessageID    string                `json:"messageID"`
		Delivery     opencode.Delivery     `json:"delivery"`
		Confirmation lifecycleConfirmation `json:"confirmation"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	if req.MessageID != "" && !validChatID(req.MessageID, "msg_") {
		writeJSON(w, 400, map[string]string{"error": "messageID must be a valid msg_ id"})
		return
	}
	if !validDelivery(req.Delivery) {
		writeJSON(w, 422, map[string]string{"error": "delivery must be queue or steer"})
		return
	}
	if _, ok := s.confirmedIdle(w, r, id, req.Confirmation); !ok {
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	item, err := s.oc.CompactSession(r.Context(), cap, id, req.MessageID, req.Delivery)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"inbox": item})
}
func (s *Server) sessionDeliver(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	req, ok := decodeChatPrompt(w, r)
	if !ok {
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if !validChatID(req.ID, "msg_") {
		writeJSON(w, 400, map[string]string{"error": "valid msg_ id is required"})
		return
	}
	if !validDelivery(req.Delivery) {
		writeJSON(w, 422, map[string]string{"error": "delivery must be queue or steer"})
		return
	}
	if len(req.Files)+len(req.References) > chatMaxFiles {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "at most 10 files are allowed"})
		return
	}
	files, err := makePromptFiles(req.Files)
	if err != nil {
		writeJSON(w, chatFileErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	refs, err := s.resolveChatReferences(r.Context(), req.References)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	files = append(files, refs...)
	skills, err := s.resolveChatSkills(r.Context(), req.Skills)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if req.Text == "" && len(files) == 0 && len(skills) == 0 {
		writeJSON(w, 422, map[string]string{"error": "text, a file, or a skill is required"})
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	item, err := s.oc.DeliverPrompt(r.Context(), cap, id, req.ID, req.Text, files, skills, req.Delivery)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"inbox": item})
}
func (s *Server) sessionInbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	items, err := s.oc.ListInbox(r.Context(), cap, id)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	filtered := items[:0]
	for _, item := range items {
		if item.SessionID == id && validChatID(item.ID, "msg_") {
			filtered = append(filtered, item)
		}
	}
	writeJSON(w, 200, map[string]any{"items": filtered, "capabilities": cap})
}
func (s *Server) sessionInboxDelivery(w http.ResponseWriter, r *http.Request) {
	id, iid := r.PathValue("sessionID"), r.PathValue("inboxID")
	if !s.chatMutationReady(w, id, iid, "msg_") {
		return
	}
	var req struct {
		Delivery opencode.Delivery `json:"delivery"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	if !validDelivery(req.Delivery) {
		writeJSON(w, 422, map[string]string{"error": "delivery must be queue or steer"})
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	if err := s.oc.ChangeInboxDelivery(r.Context(), cap, id, iid, req.Delivery); err != nil {
		writeLifecycleError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) sessionInboxCancel(w http.ResponseWriter, r *http.Request) {
	id, iid := r.PathValue("sessionID"), r.PathValue("inboxID")
	if !s.chatMutationReady(w, id, iid, "msg_") {
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	if err := s.oc.CancelInbox(r.Context(), cap, id, iid); err != nil {
		writeLifecycleError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) sessionChildren(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	if !onlyChatQueries(w, r, "cursor", "limit") {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, 400, map[string]string{"error": "limit must be 1-100"})
			return
		}
		limit = n
	}
	cursor := r.URL.Query().Get("cursor")
	if len(cursor) > 4096 {
		writeJSON(w, 400, map[string]string{"error": "cursor is too long"})
		return
	}
	page, err := s.oc.ListChildrenPage(r.Context(), id, limit, cursor)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	writeJSON(w, 200, page)
}
func (s *Server) sessionRename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unavailable; rename was not attempted"})
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > 256 {
		writeJSON(w, 422, map[string]string{"error": "title must be 1-256 characters"})
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	if err := s.oc.RenameSessionCompatible(r.Context(), cap, id, req.Title); err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	if err := s.sessions.updateTitle(id, req.Title); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "OpenCode session renamed, but the fallback mapping title could not be saved; retry rename to reconcile it",
			"renamed": true, "mappingSaved": false,
		})
		return
	}
	w.WriteHeader(204)
}
func (s *Server) sessionExport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	result, err := s.oc.ExportSessionSanitized(r.Context(), cap, id)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, safeSessionExportFilename(id)))
	writeJSON(w, 200, result)
}

func safeSessionExportFilename(id string) string {
	var safe strings.Builder
	for _, r := range id {
		if safe.Len() >= 128 {
			break
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			safe.WriteRune(r)
		} else {
			safe.WriteByte('_')
		}
	}
	return "session-" + safe.String() + ".json"
}

type sessionDeleteNode struct {
	SessionID string `json:"sessionID"`
	ParentID  string `json:"parentID,omitempty"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Depth     int    `json:"depth"`
}

type sessionDeletePreview struct {
	SessionID   string               `json:"sessionID"`
	Title       string               `json:"title"`
	Count       int                  `json:"count"`
	Fingerprint string               `json:"fingerprint"`
	Sessions    []sessionDeleteNode  `json:"sessions"`
	Mappings    []mappedSessionOwner `json:"mappings"`
}

// deletePreview reads the complete parentID tree without the fail-open
// behavior used by navigation. A destructive confirmation must never omit a
// branch because one child page failed.
func (s *Server) deletePreview(ctx context.Context, id string) (sessionDeletePreview, error) {
	root, err := s.oc.GetSession(ctx, id)
	if err != nil {
		return sessionDeletePreview{}, err
	}
	preview := sessionDeletePreview{SessionID: id, Title: root.Title}
	preview.Sessions = append(preview.Sessions, sessionDeleteNode{SessionID: root.ID, ParentID: root.ParentID, Title: root.Title, Updated: root.Time.Updated})
	seen := map[string]bool{id: true}
	for at := 0; at < len(preview.Sessions); at++ {
		parent := preview.Sessions[at]
		cursor := ""
		cursors := map[string]bool{}
		for {
			page, pageErr := s.oc.ListChildrenPage(ctx, parent.SessionID, 100, cursor)
			if pageErr != nil {
				return sessionDeletePreview{}, pageErr
			}
			for _, child := range page.Sessions {
				if child.ID == "" || child.ParentID != parent.SessionID || seen[child.ID] {
					continue
				}
				seen[child.ID] = true
				preview.Sessions = append(preview.Sessions, sessionDeleteNode{SessionID: child.ID, ParentID: child.ParentID, Title: child.Title, Updated: child.Time.Updated, Depth: parent.Depth + 1})
			}
			next := page.Cursor.Next
			if next == "" {
				break
			}
			if next == cursor || cursors[next] {
				return sessionDeletePreview{}, errors.New("OpenCode returned a repeated child-session cursor")
			}
			cursors[next] = true
			cursor = next
		}
	}
	preview.Count = len(preview.Sessions)
	preview.Mappings = s.sessions.owners(seen)
	fingerprintNodes := append([]sessionDeleteNode(nil), preview.Sessions...)
	sort.Slice(fingerprintNodes, func(i, j int) bool { return fingerprintNodes[i].SessionID < fingerprintNodes[j].SessionID })
	encoded, _ := json.Marshal(struct {
		Nodes    []sessionDeleteNode
		Mappings []mappedSessionOwner
	}{fingerprintNodes, preview.Mappings})
	sum := sha256.Sum256(encoded)
	preview.Fingerprint = hex.EncodeToString(sum[:])
	return preview, nil
}

func (s *Server) sessionDeletePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	if s.mapErr != nil {
		writeJSON(w, 503, map[string]string{"error": "session mapping unavailable"})
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	if !cap.Delete {
		writeLifecycleError(w, opencode.ErrCapabilityUnavailable)
		return
	}
	preview, err := s.deletePreview(r.Context(), id)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) sessionUnlink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !validChatID(id, "ses_") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ses_ session id required"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, 503, map[string]string{"error": "session mapping unavailable"})
		return
	}
	if err := s.sessions.removeEverywhere(map[string]bool{id: true}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "mapping unlink failed; the OpenCode session remains and unlink can be retried"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sessionDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	var req struct {
		Confirmation struct {
			SessionID   string `json:"sessionID"`
			Fingerprint string `json:"fingerprint"`
			Count       int    `json:"count"`
			Title       string `json:"title"`
		} `json:"confirmation"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	if s.mapErr != nil {
		writeJSON(w, 503, map[string]string{"error": "session mapping unavailable"})
		return
	}
	cap, ok := s.lifecycleCapabilities(w, r)
	if !ok {
		return
	}
	if !cap.Delete {
		writeLifecycleError(w, opencode.ErrCapabilityUnavailable)
		return
	}
	if req.Confirmation.SessionID != id || req.Confirmation.Fingerprint == "" || req.Confirmation.Count < 1 || req.Confirmation.Title == "" {
		writeJSON(w, http.StatusPreconditionRequired, map[string]string{"error": "current tree fingerprint, count, title, and sessionID confirmation are required"})
		return
	}
	preview, err := s.deletePreview(r.Context(), id)
	if err != nil {
		var apiErr *opencode.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			// A prior attempt may have deleted upstream and failed only while
			// saving mappings. Reconcile the persisted Parent closure safely.
			if cleanupErr := s.sessions.removeEverywhere(s.sessions.mappingClosure(id)); cleanupErr != nil {
				writeJSON(w, 500, map[string]any{"error": "OpenCode session is already absent, but mapping cleanup still failed; retry deletion", "deleted": true, "mappingSaved": false, "recoverable": true})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeChatUpstreamError(w, err)
		return
	}
	if preview.Fingerprint != req.Confirmation.Fingerprint || preview.Count != req.Confirmation.Count || preview.Title != req.Confirmation.Title {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "session tree changed after confirmation; refresh the deletion preview and retry"})
		return
	}
	active, err := s.oc.ListActiveSessions(r.Context())
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	deleted := make(map[string]bool, preview.Count)
	for _, node := range preview.Sessions {
		deleted[node.SessionID] = true
		if active[node.SessionID].Type == "running" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "session tree contains a busy session; refresh and retry when all sessions are idle"})
			return
		}
	}
	if err := s.oc.DeleteSession(r.Context(), id); err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	if err := s.sessions.removeEverywhere(deleted); err != nil {
		writeJSON(w, 500, map[string]any{"error": "OpenCode session and descendants were deleted, but mapping cleanup failed; retry deletion to reconcile mappings", "deleted": true, "mappingSaved": false, "recoverable": true})
		return
	}
	w.WriteHeader(204)
}
