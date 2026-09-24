package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"lessmess/internal/opencode"
)

const (
	chatBodyLimit      = 1 << 20
	chatPromptLimit    = 28 << 20
	chatMessagePage    = 50
	chatMaxFiles       = 10
	chatMaxFileSize    = 20 << 20
	chatMaxFileTotal   = 20 << 20
	chatDetailChunk    = 32 << 10
	chatDetailMax      = 1 << 20
	chatInputMax       = 16 << 10
	chatDiffFileMax    = 80 << 10
	chatDiffTotalMax   = 160 << 10
	chatToolFilesMax   = 32
	chatShellOutputMax = 32 << 10
)

type chatSnapshotView struct {
	SessionID   string
	Busy        bool
	History     bool
	OlderCursor string
	Blocks      []chatTranscriptBlockView
	Permissions []opencode.PermissionRequest
	Forms       []chatFormView
}

type chatTranscriptBlockView struct {
	Kind      string
	SourceIDs []string
	Markers   []chatMessageMarkerView
	Message   *chatMessageView
	Activity  []chatActivityView
	ExpandKey string
	Summary   string
	Running   bool
}

type chatMessageMarkerView struct {
	ID, Type, Status string
}

type chatActivityView struct {
	Kind, ExpandKey string
	Part            *chatPartView
	Shell           *chatShellView
	Running         bool
}

type chatMessageView struct {
	ID        string
	Type      string
	Status    string
	Label     string
	Text      template.HTML
	Markdown  string
	Copyable  bool
	Parts     []chatPartView
	Files     []chatFileView
	Error     string
	DiffURL   string
	Shell     *chatShellView
	Completed bool
}

type chatShellView struct {
	Command, Status, Exit, Output string
	Truncated                     bool
}

type chatPartView struct {
	Kind        string
	Text        template.HTML
	Markdown    string
	ToolName    string
	ToolStatus  string
	ToolID      string
	ToolURL     string
	UnknownType string
}

type chatToolDetailView struct {
	Name, Status, Input, Output, Error, Timing string
	Files                                      []chatToolFileView
	NextURL                                    string
	Truncated                                  bool
}

type chatToolFileView struct{ Name, MIME, URI string }

type chatDiffView struct {
	Files                []chatDiffFileView
	Additions, Deletions int
	Truncated            bool
}

type chatDiffFileView struct {
	File, Status         string
	Additions, Deletions int
	Lines                []chatDiffLineView
	Fallback             string
	Truncated            bool
}

type chatDiffLineView struct{ Kind, Text string }

type chatFileView struct {
	Name      string
	MIME      string
	URL       string
	Image     bool
	Reference bool
}

type chatPromptFile struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data string `json:"data"`
}

type chatPromptRequest struct {
	ID         string            `json:"id,omitempty"`
	Text       string            `json:"text"`
	Files      []chatPromptFile  `json:"files,omitempty"`
	References []string          `json:"references,omitempty"`
	Skills     []string          `json:"skills,omitempty"`
	Delivery   opencode.Delivery `json:"delivery,omitempty"`
}

type chatReferenceView struct {
	Alias       string `json:"alias"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type chatControlsResponse struct {
	Agents          []settingsAgentOpt     `json:"agents"`
	Models          []chatModelOpt         `json:"models"`
	Commands        []opencode.CommandInfo `json:"commands"`
	Skills          []opencode.SkillInfo   `json:"skills"`
	Agent           string                 `json:"agent,omitempty"`
	Model           string                 `json:"model,omitempty"`
	Variant         string                 `json:"variant,omitempty"`
	StandaloneSkill bool                   `json:"standaloneSkill"`
	Usage           chatUsageView          `json:"usage"`
}

type chatModelOpt struct {
	settingsModelOpt
	Variants []string `json:"variants"`
}

type chatUsageView struct {
	Input            int  `json:"input"`
	Output           int  `json:"output"`
	Reasoning        int  `json:"reasoning"`
	CacheRead        int  `json:"cacheRead"`
	CacheWrite       int  `json:"cacheWrite"`
	ContextLimit     int  `json:"contextLimit"`
	ContextAvailable bool `json:"contextAvailable"`
	EstimatedContext int  `json:"estimatedContext"`
	Percent          int  `json:"percent"`
	Warning          bool `json:"warning"`
}

type chatFormView struct {
	ID     string
	Title  string
	Fields []chatFormFieldView
}

type chatFormFieldView struct {
	Key         string
	Type        string
	Title       string
	Description string
	Required    bool
	URL         string
	Placeholder string
	Default     string
	Checked     bool
	Minimum     string
	Maximum     string
	Options     []chatFormOptionView
}

type chatFormOptionView struct {
	Value       string
	Label       string
	Description string
	Selected    bool
}

func validChatID(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) <= len(prefix) {
		return false
	}
	return !strings.ContainsAny(value, "/\\")
}

func (s *Server) chatSnapshot(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !validChatID(sessionID, "ses_") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ses_ session id required"})
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}

	cursors := r.URL.Query()["cursor"]
	if len(cursors) > 1 || (len(cursors) == 1 && (cursors[0] == "" || len(cursors[0]) > 4096)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "one non-empty cursor is required"})
		return
	}
	var cursor string
	if len(cursors) == 1 {
		cursor = cursors[0]
	}
	opts := opencode.ListMessagesOptions{Limit: chatMessagePage, Cursor: cursor}
	if cursor == "" {
		opts.Order = "desc"
	}
	page, err := s.oc.ListMessagesPage(r.Context(), sessionID, opts)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	view := chatSnapshotView{
		SessionID: sessionID,
		History:   cursor != "",
		// A descending timeline advances toward older messages with next.
		OlderCursor: page.Cursor.Next,
	}
	var messages []chatMessageView
	for i := len(page.Messages) - 1; i >= 0; i-- {
		messages = append(messages, makeChatMessageView(sessionID, page.Messages[i]))
	}
	view.Blocks = makeChatTranscriptBlocks(messages)
	if cursor != "" {
		s.rend.render(w, s.rend.partial, "chatSnapshot", view)
		return
	}
	active, err := s.oc.ListActiveSessions(r.Context())
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	permissions, err := s.oc.ListPermissions(r.Context(), sessionID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	forms, err := s.oc.ListForms(r.Context(), sessionID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}

	view.Permissions = permissions
	view.Busy = active[sessionID].Type == "running"
	for _, form := range forms {
		view.Forms = append(view.Forms, makeChatFormView(form))
	}
	markRunningChatActivity(view.Blocks, view.Busy, len(view.Permissions) > 0 || len(view.Forms) > 0)
	s.rend.render(w, s.rend.partial, "chatSnapshot", view)
}

func markRunningChatActivity(blocks []chatTranscriptBlockView, busy, waitingForInput bool) {
	if busy && !waitingForInput && len(blocks) > 0 {
		latest := &blocks[len(blocks)-1]
		if latest.Kind == "activity" && len(latest.Activity) > 0 && latest.Activity[len(latest.Activity)-1].Running {
			latest.Running = true
			latest.Summary = runningActivitySummary(latest.Activity[len(latest.Activity)-1])
		}
	}
}

func makeChatTranscriptBlocks(messages []chatMessageView) []chatTranscriptBlockView {
	var blocks []chatTranscriptBlockView
	var activity *chatTranscriptBlockView
	marked := make(map[string]bool)

	flushActivity := func() { activity = nil }
	addMarker := func(block *chatTranscriptBlockView, message chatMessageView) {
		if marked[message.ID] {
			return
		}
		marked[message.ID] = true
		block.Markers = append(block.Markers, chatMessageMarkerView{ID: message.ID, Type: message.Type, Status: message.Status})
	}
	addMessage := func(message chatMessageView) {
		flushActivity()
		block := chatTranscriptBlockView{Kind: "message", SourceIDs: []string{message.ID}, Message: &message}
		addMarker(&block, message)
		blocks = append(blocks, block)
	}
	addActivity := func(message chatMessageView, item chatActivityView) {
		if activity == nil {
			blocks = append(blocks, chatTranscriptBlockView{
				Kind: "activity", ExpandKey: "system:" + item.ExpandKey, Summary: "System",
			})
			activity = &blocks[len(blocks)-1]
		}
		if len(activity.SourceIDs) == 0 || activity.SourceIDs[len(activity.SourceIDs)-1] != message.ID {
			activity.SourceIDs = append(activity.SourceIDs, message.ID)
		}
		addMarker(activity, message)
		activity.Activity = append(activity.Activity, item)
	}

	for _, message := range messages {
		emitted := false
		switch message.Type {
		case "synthetic":
			continue
		case "assistant":
			if message.Text != "" {
				chunk := chatMessageView{ID: message.ID, Type: message.Type, Status: message.Status, Label: message.Label, Text: message.Text, Markdown: message.Markdown, Copyable: message.Markdown != ""}
				addMessage(chunk)
				emitted = true
			}
			if len(message.Files) > 0 {
				chunk := chatMessageView{ID: message.ID, Type: message.Type, Status: message.Status, Label: message.Label, Files: message.Files}
				addMessage(chunk)
				emitted = true
			}
			for i := range message.Parts {
				part := message.Parts[i]
				switch part.Kind {
				case "reasoning", "tool":
					key := part.Kind + ":" + message.ID + ":" + strconv.Itoa(i)
					if part.Kind == "tool" && part.ToolID != "" {
						key = "tool:" + message.ID + ":" + part.ToolID
					}
					running := part.Kind == "reasoning" && !message.Completed
					if part.Kind == "tool" {
						running = part.ToolStatus == "running" || part.ToolStatus == "streaming"
					}
					addActivity(message, chatActivityView{Kind: part.Kind, ExpandKey: key, Part: &part, Running: running})
				default:
					chunk := chatMessageView{ID: message.ID, Type: message.Type, Status: message.Status, Label: message.Label, Parts: []chatPartView{part}, Copyable: part.Kind == "text" && part.Markdown != ""}
					addMessage(chunk)
				}
				emitted = true
			}
			if message.Error != "" {
				chunk := chatMessageView{ID: message.ID, Type: message.Type, Status: message.Status, Label: message.Label, Error: message.Error}
				addMessage(chunk)
				emitted = true
			}
		case "shell":
			if message.Shell != nil {
				addActivity(message, chatActivityView{Kind: "shell", ExpandKey: "shell:" + message.ID, Shell: message.Shell, Running: message.Status == "running" || message.Status == "streaming"})
				emitted = true
			}
			if message.Error != "" {
				chunk := chatMessageView{ID: message.ID, Type: message.Type, Status: message.Status, Label: message.Label, Error: message.Error}
				addMessage(chunk)
				emitted = true
			}
		default:
			addMessage(message)
			emitted = true
		}
		if !emitted {
			addMessage(message)
		}
	}
	return blocks
}

func runningActivitySummary(activity chatActivityView) string {
	switch activity.Kind {
	case "reasoning":
		return "Thinking"
	case "shell":
		return "Running shell"
	case "tool":
		name := ""
		if activity.Part != nil {
			name, _ = sliceChatBytes(strings.TrimSpace(activity.Part.ToolName), 0, 64)
		}
		if name != "" {
			return "Running " + name
		}
		return "Running tool"
	default:
		return "System"
	}
}

func makeChatMessageView(sessionID string, message opencode.Message) chatMessageView {
	view := chatMessageView{ID: message.ID, Type: message.Type, Status: message.Status, Label: message.Type, Completed: message.Time.Completed > 0}
	switch message.Type {
	case "user":
		view.Label = "You"
		view.Text = renderMarkdown(message.Text)
		view.Markdown = message.Text
		view.Copyable = true
	case "assistant":
		view.Label = "Assistant"
		if message.Retry != nil {
			view.Markdown = fmt.Sprintf("Retrying provider request (attempt %d).", message.Retry.Attempt)
			view.Text = renderMarkdown(view.Markdown)
		}
		if message.Finish == "length" {
			view.Error = "The model stopped because its output limit was reached. Continue the conversation to resume."
		} else if message.Finish == "content-filter" {
			view.Error = "The provider stopped this response because of its content policy."
		} else if message.Finish == "error" {
			view.Error = "The provider could not finish this response. Check the selected model or provider connection, then retry."
		}
	case "compaction":
		view.Label = "Context compaction"
		switch message.Status {
		case "running":
			view.Text = renderMarkdown("Compacting conversation context...")
		case "completed":
			view.Text = renderMarkdown("Conversation context compacted.")
		case "failed":
			view.Error = "Context compaction failed. You can continue and retry later."
		default:
			view.Text = renderMarkdown("Conversation context changed.")
		}
	case "shell":
		view.Label = "Shell operation"
		command, _ := sliceChatBytes(message.Command, 0, chatInputMax)
		output := ""
		outputTruncated := false
		if message.Output != nil {
			output, _ = sliceChatBytes(message.Output.Output, 0, chatShellOutputMax)
			outputTruncated = message.Output.Truncated || len(output) < len(message.Output.Output)
		}
		view.Shell = &chatShellView{
			Command: command, Status: firstNonempty(message.Status, "unknown"),
			Exit: formatShellExit(message.Exit), Output: output,
			Truncated: len(command) < len(message.Command) || outputTruncated,
		}
	case "system":
		view.Label = "System"
		view.Text = renderMarkdown(firstNonempty(message.Text, message.Description))
	default:
		view.Text = renderMarkdown(firstNonempty(message.Text, message.Description))
	}
	if message.Error != nil && view.Error == "" {
		view.Error = safeChatStructuredError(message.Error)
	}
	for _, part := range message.Content {
		pv := chatPartView{Kind: part.PartType()}
		switch part := part.(type) {
		case opencode.TextPart:
			pv.Text = renderMarkdown(part.Text)
			pv.Markdown = part.Text
		case opencode.ReasoningPart:
			pv.Text = renderMarkdown(part.Text)
		case opencode.ToolPart:
			pv.ToolName, _ = sliceChatBytes(part.Name, 0, 256)
			pv.ToolStatus = normalizeToolStatus(part.State.Status)
			pv.ToolID = part.ID
			pv.ToolURL = fmt.Sprintf("/api/sessions/%s/chat/messages/%s/tools/%s", url.PathEscape(sessionID), url.PathEscape(message.ID), url.PathEscape(part.ID))
		case opencode.UnknownPart:
			pv.Kind = "unknown"
			pv.UnknownType = firstNonempty(part.Type, "unknown")
		default:
			pv.Kind = "unknown"
			pv.UnknownType = firstNonempty(part.PartType(), "unknown")
		}
		view.Parts = append(view.Parts, pv)
	}
	for i, file := range message.Files {
		mimeType, _, _ := mime.ParseMediaType(file.MIME)
		view.Files = append(view.Files, chatFileView{
			Name: safeChatFilename(file.Name), MIME: mimeType,
			URL:   fmt.Sprintf("/api/sessions/%s/chat/messages/%s/files/%d", url.PathEscape(sessionID), url.PathEscape(message.ID), i),
			Image: isChatImage(mimeType), Reference: file.Source.Type == "uri",
		})
	}
	return view
}

func formatShellExit(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "unknown"
	}
	switch value := value.(type) {
	case string:
		text, _ := sliceChatBytes(value, 0, 128)
		return text
	case float64, bool:
		return fmt.Sprint(value)
	default:
		return "reported"
	}
}

func safeChatStructuredError(err *opencode.StructuredError) string {
	if err == nil {
		return ""
	}
	kind := strings.ToLower(err.Type)
	switch {
	case err.Status == http.StatusUnauthorized || err.Status == http.StatusForbidden || strings.Contains(kind, "auth"):
		return "Provider authentication failed. Reconnect the provider in OpenCode and retry."
	case err.Status == http.StatusTooManyRequests || strings.Contains(kind, "rate"):
		return "The provider rate limit was reached. Wait briefly, then retry."
	case strings.Contains(kind, "provider"):
		return "The provider request failed. Check the selected model or provider connection, then retry."
	default:
		return "OpenCode could not complete this response. Retry or check the session controls."
	}
}

func normalizeToolStatus(status string) string {
	switch status {
	case "streaming", "running", "completed", "error":
		return status
	case "":
		return "unknown"
	default:
		return "unknown (" + status + ")"
	}
}

func makeChatFormView(form opencode.Form) chatFormView {
	view := chatFormView{ID: form.ID, Title: form.Title}
	for _, field := range form.Fields {
		if field.Hidden {
			continue
		}
		fv := chatFormFieldView{
			Key: field.Key, Type: field.Type, Title: firstNonempty(field.Title, field.Key),
			Description: field.Description, Required: field.Required, URL: field.URL,
			Placeholder: field.Placeholder, Default: rawFormString(field.Default),
			Minimum: rawFormString(field.Minimum), Maximum: rawFormString(field.Maximum),
		}
		_ = json.Unmarshal(field.Default, &fv.Checked)
		var selected []string
		_ = json.Unmarshal(field.Default, &selected)
		selectedSet := make(map[string]bool, len(selected))
		for _, value := range selected {
			selectedSet[value] = true
		}
		for _, option := range field.Options {
			fv.Options = append(fv.Options, chatFormOptionView{
				Value: option.Value, Label: option.Label, Description: option.Description,
				Selected: option.Value == fv.Default || selectedSet[option.Value],
			})
		}
		view.Fields = append(view.Fields, fv)
	}
	return view
}

func rawFormString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	return string(raw)
}

func (s *Server) chatToolDetail(w http.ResponseWriter, r *http.Request) {
	sessionID, messageID, toolID := r.PathValue("sessionID"), r.PathValue("messageID"), r.PathValue("toolID")
	if !chatReadReady(w, s.oc, sessionID, messageID, toolID) {
		return
	}
	if !onlyChatQueries(w, r, "offset", "limit") {
		return
	}
	offset, limit, ok := chatRangeQuery(w, r, chatDetailChunk, chatDetailChunk)
	if !ok {
		return
	}
	message, err := s.oc.GetMessage(r.Context(), sessionID, messageID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	var tool *opencode.ToolPart
	for _, part := range message.Content {
		if value, match := part.(opencode.ToolPart); match && value.ID == toolID {
			value := value
			tool = &value
			break
		}
	}
	if tool == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tool call not found in message"})
		return
	}

	view := chatToolDetailView{Name: tool.Name, Status: normalizeToolStatus(tool.State.Status)}
	view.Input = formatToolInput(tool.State.Input, chatInputMax)
	if tool.State.Error != nil {
		view.Error, _ = sliceChatBytes(tool.State.Error.Message, 0, chatInputMax)
	}
	var output strings.Builder
	for _, content := range tool.State.Content {
		switch content.Type {
		case "text":
			if output.Len() >= chatDetailMax {
				view.Truncated = true
				continue
			}
			separator := ""
			if output.Len() > 0 && content.Text != "" {
				separator = "\n"
			}
			piece := separator + content.Text
			available := chatDetailMax - output.Len()
			if len(piece) > available {
				piece, _ = sliceChatBytes(piece, 0, available)
				view.Truncated = true
			}
			output.WriteString(piece)
		case "file":
			if len(view.Files) < chatToolFilesMax {
				name, _ := sliceChatBytes(content.Name, 0, 256)
				mimeType, _ := sliceChatBytes(content.MIME, 0, 128)
				uri, _ := sliceChatBytes(content.URI, 0, 1024)
				view.Files = append(view.Files, chatToolFileView{Name: name, MIME: mimeType, URI: uri})
			} else {
				view.Truncated = true
			}
		}
	}
	allOutput := output.String()
	view.Output, ok = sliceChatBytes(allOutput, offset, limit)
	if !ok {
		writeJSON(w, http.StatusRequestedRangeNotSatisfiable, map[string]string{"error": "offset exceeds available tool output"})
		return
	}
	next := offset + len(view.Output)
	if next < len(allOutput) {
		view.NextURL = fmt.Sprintf("/api/sessions/%s/chat/messages/%s/tools/%s?offset=%d&limit=%d", url.PathEscape(sessionID), url.PathEscape(messageID), url.PathEscape(toolID), next, limit)
	}
	view.Timing = formatToolTiming(tool.Time)
	s.rend.render(w, s.rend.partial, "chatToolDetail", view)
}

func (s *Server) chatDiff(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !validChatID(sessionID, "ses_") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ses_ session id required"})
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if !onlyChatQueries(w, r, "from", "to") {
		return
	}
	from, ok := oneChatQuery(w, r, "from", true)
	if !ok {
		return
	}
	to, ok := oneChatQuery(w, r, "to", false)
	if !ok {
		return
	}
	if !validChatID(from, "msg_") || (to != "" && !validChatID(to, "msg_")) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to must be valid msg_ ids"})
		return
	}
	diffs, err := s.oc.GetSessionDiff(r.Context(), sessionID, from, to, 3)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	view := chatDiffView{}
	remaining := chatDiffTotalMax
	for i, diff := range diffs {
		if i >= 200 {
			view.Truncated = true
			break
		}
		fileName, _ := sliceChatBytes(diff.File, 0, 4096)
		file := chatDiffFileView{File: fileName, Status: diff.Status, Additions: diff.Additions, Deletions: diff.Deletions}
		view.Additions += diff.Additions
		view.Deletions += diff.Deletions
		budget := chatDiffFileMax
		if remaining < budget {
			budget = remaining
		}
		patch, _ := sliceChatBytes(diff.Patch, 0, budget)
		file.Truncated = len(diff.Patch) > len(patch)
		if budget == 0 {
			file.Truncated = true
			view.Truncated = true
		} else if diff.Patch == "" {
			file.Fallback = "No text patch is available for this file."
		} else if strings.Contains(diff.Patch, "GIT binary patch") || strings.Contains(diff.Patch, "Binary files ") {
			file.Fallback = "Binary file changed; no readable text patch is available."
		} else {
			file.Lines = makeDiffLines(patch)
		}
		remaining -= len(patch)
		if file.Truncated {
			view.Truncated = true
		}
		view.Files = append(view.Files, file)
	}
	s.rend.render(w, s.rend.partial, "chatDiff", view)
}

func chatReadReady(w http.ResponseWriter, client *opencode.Client, sessionID, messageID, toolID string) bool {
	if !validChatID(sessionID, "ses_") || !validChatID(messageID, "msg_") || toolID == "" || len(toolID) > 512 || strings.ContainsAny(toolID, "/\\") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid session, message, and tool ids required"})
		return false
	}
	if client == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return false
	}
	return true
}

func oneChatQuery(w http.ResponseWriter, r *http.Request, key string, required bool) (string, bool) {
	values := r.URL.Query()[key]
	if len(values) > 1 || (len(values) == 1 && (values[0] == "" || len(values[0]) > 512)) || (required && len(values) == 0) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "one valid " + key + " query value is required"})
		return "", false
	}
	if len(values) == 0 {
		return "", true
	}
	return values[0], true
}

func onlyChatQueries(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	set := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		set[key] = true
	}
	for key := range r.URL.Query() {
		if !set[key] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported query parameter: " + key})
			return false
		}
	}
	return true
}

func chatRangeQuery(w http.ResponseWriter, r *http.Request, defaultLimit, maxLimit int) (int, int, bool) {
	parse := func(key string, fallback int) (int, bool) {
		values := r.URL.Query()[key]
		if len(values) == 0 {
			return fallback, true
		}
		if len(values) != 1 {
			return 0, false
		}
		value, err := strconv.Atoi(values[0])
		return value, err == nil && value >= 0
	}
	offset, okOffset := parse("offset", 0)
	limit, okLimit := parse("limit", defaultLimit)
	if !okOffset || !okLimit || limit < 1 || limit > maxLimit || offset > chatDetailMax {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("offset must be 0-%d and limit must be 1-%d", chatDetailMax, maxLimit)})
		return 0, 0, false
	}
	return offset, limit, true
}

func sliceChatBytes(value string, offset, limit int) (string, bool) {
	if offset > len(value) {
		return "", false
	}
	end := offset + limit
	if end > len(value) {
		end = len(value)
	}
	for offset < len(value) && !utf8.RuneStart(value[offset]) {
		offset++
	}
	if offset >= end {
		return "", true
	}
	for end > offset && end < len(value) && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[offset:end], true
}

func formatToolInput(raw json.RawMessage, limit int) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw) > limit {
		value, _ := sliceChatBytes(string(raw), 0, limit)
		return value + "\n[input truncated]"
	}
	var streaming string
	if json.Unmarshal(raw, &streaming) == nil {
		return streaming
	}
	var formatted bytes.Buffer
	if json.Indent(&formatted, raw, "", "  ") == nil {
		return formatted.String()
	}
	return "[invalid tool input]"
}

func formatToolTiming(value opencode.MessageTime) string {
	parts := []string{fmt.Sprintf("created %.0f", value.Created)}
	if value.Ran > 0 {
		parts = append(parts, fmt.Sprintf("started +%.0fms", value.Ran-value.Created))
	}
	if value.Completed > 0 {
		start := value.Ran
		if start == 0 {
			start = value.Created
		}
		parts = append(parts, fmt.Sprintf("finished +%.0fms", value.Completed-start))
	}
	return strings.Join(parts, ", ")
}

func makeDiffLines(patch string) []chatDiffLineView {
	lines := strings.Split(strings.TrimSuffix(patch, "\n"), "\n")
	result := make([]chatDiffLineView, 0, len(lines))
	for _, line := range lines {
		kind := "context"
		switch {
		case strings.HasPrefix(line, "@@"):
			kind = "hunk"
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			kind = "add"
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			kind = "delete"
		case strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
			kind = "meta"
		}
		result = append(result, chatDiffLineView{Kind: kind, Text: line})
	}
	return result
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func decodeChatJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	return decodeChatJSONLimit(w, r, out, chatBodyLimit)
}

func decodeChatJSONLimit(w http.ResponseWriter, r *http.Request, out any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body is too large"})
			return false
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must contain one JSON object"})
		return false
	}
	return true
}

func (s *Server) chatPrompt(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	req, ok := decodeChatPrompt(w, r)
	if !ok {
		return
	}
	req.Text = strings.TrimSpace(req.Text)
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
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "text, a file, or a skill is required"})
		return
	}
	if err := s.oc.PromptWithFilesAndSkills(r.Context(), sessionID, req.Text, files, skills); err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

var errChatFileTooLarge = errors.New("file limits exceeded")

func decodeChatPrompt(w http.ResponseWriter, r *http.Request) (chatPromptRequest, bool) {
	var req chatPromptRequest
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid content type"})
		return req, false
	}
	if mediaType == "application/json" {
		if !decodeChatJSONLimit(w, r, &req, chatPromptLimit) {
			return req, false
		}
		return req, true
	}
	if mediaType != "multipart/form-data" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "application/json or multipart/form-data required"})
		return req, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, chatMaxFileTotal+(1<<20))
	mr, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart body"})
		return req, false
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeChatMultipartError(w, err)
			return req, false
		}
		data, readErr := io.ReadAll(io.LimitReader(part, chatMaxFileSize+1))
		part.Close()
		if readErr != nil {
			writeChatMultipartError(w, readErr)
			return req, false
		}
		switch part.FormName() {
		case "text":
			if part.FileName() != "" || len(data) > 1<<20 {
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "prompt text is too large"})
				return req, false
			}
			req.Text = string(data)
		case "id":
			if part.FileName() != "" || len(data) > 256 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid message id"})
				return req, false
			}
			req.ID = string(data)
		case "delivery":
			if part.FileName() != "" || len(data) > 16 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid delivery mode"})
				return req, false
			}
			req.Delivery = opencode.Delivery(data)
		case "references":
			if part.FileName() != "" || len(data) > 256 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reference alias"})
				return req, false
			}
			req.References = append(req.References, string(data))
		case "skills":
			if part.FileName() != "" || len(data) > 256 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill id"})
				return req, false
			}
			req.Skills = append(req.Skills, string(data))
		case "files":
			if part.FileName() == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file name is required"})
				return req, false
			}
			req.Files = append(req.Files, chatPromptFile{Name: part.FileName(), MIME: part.Header.Get("Content-Type"), Data: base64.StdEncoding.EncodeToString(data)})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown multipart field"})
			return req, false
		}
	}
	return req, true
}

func writeChatMultipartError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "attachment total exceeds 20 MiB"})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart body"})
}

func makePromptFiles(inputs []chatPromptFile) ([]opencode.PromptFile, error) {
	if len(inputs) > chatMaxFiles {
		return nil, errChatFileTooLarge
	}
	files := make([]opencode.PromptFile, 0, len(inputs))
	total := 0
	for _, input := range inputs {
		if len(input.Data) > base64.StdEncoding.EncodedLen(chatMaxFileSize) {
			return nil, errChatFileTooLarge
		}
		data, err := base64.StdEncoding.DecodeString(input.Data)
		if err != nil {
			return nil, errors.New("file data must be base64")
		}
		total += len(data)
		if len(data) > chatMaxFileSize || total > chatMaxFileTotal {
			return nil, errChatFileTooLarge
		}
		mimeType, err := validateChatFile(input.MIME, data)
		if err != nil {
			return nil, err
		}
		files = append(files, opencode.PromptFile{
			Name: safeChatFilename(input.Name),
			URI:  "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data),
		})
	}
	return files, nil
}

func chatFileErrorStatus(err error) int {
	if errors.Is(err, errChatFileTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusUnprocessableEntity
}

func validateChatFile(contentType string, data []byte) (string, error) {
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mimeType == "" {
		return "", errors.New("file has an unsupported content type")
	}
	mimeType = strings.ToLower(mimeType)
	if mimeType == "image/svg+xml" {
		if !utf8.Valid(data) {
			return "", errors.New("SVG files must be UTF-8 text")
		}
		return mimeType, nil
	}
	if isChatImage(mimeType) {
		detected := strings.TrimSuffix(http.DetectContentType(data), "; charset=utf-8")
		if detected != mimeType {
			return "", errors.New("file content does not match its image type")
		}
		return mimeType, nil
	}
	if strings.HasPrefix(mimeType, "text/") || mimeType == "application/json" || mimeType == "application/xml" || mimeType == "application/javascript" || mimeType == "application/yaml" || mimeType == "application/toml" {
		if !utf8.Valid(data) {
			return "", errors.New("text files must be valid UTF-8")
		}
		return mimeType, nil
	}
	return "", errors.New("file has an unsupported content type")
}

func isChatImage(mimeType string) bool {
	return mimeType == "image/png" || mimeType == "image/jpeg" || mimeType == "image/gif" || mimeType == "image/webp"
}

func safeChatFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "attachment"
	}
	return name
}

func chatReferenceAlias(ref opencode.Reference) string {
	sum := sha256.Sum256([]byte(ref.Name + "\x00" + ref.Path + "\x00" + ref.Description))
	return base64.RawURLEncoding.EncodeToString(sum[:18])
}

func (s *Server) chatReferences(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	refs, err := s.oc.ListReferencesFor(r.Context(), s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	views := make([]chatReferenceView, 0, len(refs))
	for _, ref := range refs {
		if ref.Hidden {
			continue
		}
		if _, err := s.chatReferencePath(ref.Path); err != nil {
			continue
		}
		views = append(views, chatReferenceView{Alias: chatReferenceAlias(ref), Name: ref.Name, Description: ref.Description})
	}
	writeJSON(w, http.StatusOK, map[string]any{"references": views})
}

func (s *Server) resolveChatSkills(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	available, err := s.oc.ListSkillsFor(ctx, s.st.Dir)
	if err != nil {
		return nil, errors.New("could not refresh project skills")
	}
	valid := make(map[string]bool, len(available))
	for _, skill := range available {
		valid[skill.ID] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !valid[id] {
			return nil, errors.New("a selected skill is no longer available")
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *Server) resolveChatReferences(ctx context.Context, aliases []string) ([]opencode.PromptFile, error) {
	if len(aliases) == 0 {
		return nil, nil
	}
	refs, err := s.oc.ListReferencesFor(ctx, s.st.Dir)
	if err != nil {
		return nil, fmt.Errorf("refresh project references: %w", err)
	}
	available := make(map[string]opencode.Reference, len(refs))
	for _, ref := range refs {
		if !ref.Hidden {
			available[chatReferenceAlias(ref)] = ref
		}
	}
	seen := make(map[string]bool, len(aliases))
	files := make([]opencode.PromptFile, 0, len(aliases))
	for _, alias := range aliases {
		ref, ok := available[alias]
		if !ok {
			return nil, errors.New("a selected project reference is no longer available")
		}
		if seen[alias] {
			continue
		}
		seen[alias] = true
		absolute, err := s.chatReferencePath(ref.Path)
		if err != nil {
			return nil, errors.New("a selected project reference has an unsafe path")
		}
		files = append(files, opencode.PromptFile{Name: safeChatFilename(ref.Name), URI: (&url.URL{Scheme: "file", Path: absolute}).String()})
	}
	return files, nil
}

func (s *Server) chatReferencePath(value string) (string, error) {
	if value == "" || strings.IndexByte(value, 0) >= 0 {
		return "", errors.New("empty path")
	}
	absolute := value
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(s.st.Dir, filepath.FromSlash(value))
	}
	absolute, err := filepath.Abs(absolute)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(s.st.Dir)
	if err != nil {
		return "", err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, absolute)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path is outside project")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("path is not a regular file")
	}
	return absolute, nil
}

func (s *Server) chatAttachment(w http.ResponseWriter, r *http.Request) {
	sessionID, messageID := r.PathValue("sessionID"), r.PathValue("messageID")
	if !s.chatMutationReady(w, sessionID, messageID, "msg_") {
		return
	}
	index, err := strconv.Atoi(r.PathValue("fileIndex"))
	if err != nil || index < 0 || index >= chatMaxFiles {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid file index required"})
		return
	}
	message, err := s.oc.GetMessage(r.Context(), sessionID, messageID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	if message.Type != "user" || index >= len(message.Files) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "attachment not found"})
		return
	}
	file := message.Files[index]
	if len(file.Data) > base64.StdEncoding.EncodedLen(chatMaxFileSize) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "historic attachment exceeds preview limit"})
		return
	}
	data, err := base64.StdEncoding.DecodeString(file.Data)
	if err != nil || len(data) > chatMaxFileSize {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "historic attachment data is invalid"})
		return
	}
	mimeType, err := validateChatFile(file.MIME, data)
	if err != nil {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "historic attachment type is unsupported"})
		return
	}
	name := safeChatFilename(file.Name)
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if mimeType == "image/svg+xml" {
		mimeType = "text/plain; charset=utf-8"
	} else if strings.HasPrefix(mimeType, "text/") {
		mimeType += "; charset=utf-8"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) chatControls(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	agents, err := listPrimaryAgents(ctx, s.oc, s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	models, err := s.oc.ListModelsFor(ctx, s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	commands, err := s.oc.ListCommandsFor(ctx, s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	skills, err := s.oc.ListSkillsFor(ctx, s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	session, err := s.oc.GetSession(ctx, sessionID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	_, standalone, _ := s.oc.StandaloneSkillRoute(ctx)
	resp := chatControlsResponse{Commands: commands, Skills: skills, Agent: session.Agent, StandaloneSkill: standalone}
	if session.Model != nil {
		resp.Variant = session.Model.Variant
	}
	for _, agent := range agents {
		resp.Agents = append(resp.Agents, settingsAgentOpt{ID: agent.ID, Name: agent.Name, Description: agent.Description})
	}
	for _, model := range models {
		if !model.IsEnabled() {
			continue
		}
		value := model.ProviderID + "/" + model.ID
		option := chatModelOpt{settingsModelOpt: settingsModelOpt{ID: model.ID, ProviderID: model.ProviderID, Name: model.Name, Value: value}}
		for _, variant := range model.Variants {
			option.Variants = append(option.Variants, variant.ID)
		}
		resp.Models = append(resp.Models, option)
		if session.Model != nil && session.Model.ProviderID == model.ProviderID && session.Model.ID == model.ID {
			resp.Model = value
		}
	}
	resp.Usage = s.chatUsage(ctx, sessionID, session, models)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) chatUsageEndpoint(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	session, err := s.oc.GetSession(ctx, sessionID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	models, err := s.oc.ListModelsFor(ctx, s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.chatUsage(ctx, sessionID, session, models))
}

func (s *Server) chatUsage(ctx context.Context, sessionID string, session *opencode.Session, models []opencode.ModelInfo) chatUsageView {
	usage := chatUsageView{
		Input: session.Tokens.Input, Output: session.Tokens.Output, Reasoning: session.Tokens.Reasoning,
		CacheRead: session.Tokens.Cache.Read, CacheWrite: session.Tokens.Cache.Write,
	}
	for _, model := range models {
		if session.Model != nil && session.Model.ProviderID == model.ProviderID && session.Model.ID == model.ID {
			usage.ContextLimit = model.Limit.Context
			break
		}
	}
	messages, err := s.oc.GetSessionContext(ctx, sessionID)
	if err != nil {
		return usage
	}
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Type != "assistant" || message.Tokens == nil {
			continue
		}
		tokens := message.Tokens
		usage.ContextAvailable = true
		usage.EstimatedContext = tokens.Input + tokens.Cache.Read + tokens.Cache.Write + tokens.Output + tokens.Reasoning
		if usage.ContextLimit > 0 {
			usage.Percent = usage.EstimatedContext * 100 / usage.ContextLimit
			usage.Warning = usage.Percent >= 80
		}
		break
	}
	return usage
}

func (s *Server) chatSwitchAgent(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	var req struct {
		Agent string `json:"agent"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	req.Agent = strings.TrimSpace(req.Agent)
	agents, err := listPrimaryAgents(r.Context(), s.oc, s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	for _, agent := range agents {
		if agent.ID == req.Agent {
			if err := s.oc.SwitchAgent(r.Context(), sessionID, req.Agent); err != nil {
				writeChatUpstreamError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "selected agent is no longer available"})
}

func (s *Server) chatSwitchModel(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	var req struct {
		Model   string `json:"model"`
		Variant string `json:"variant"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	req.Variant = strings.TrimSpace(req.Variant)
	models, err := s.oc.ListModelsFor(r.Context(), s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	for _, model := range models {
		if model.IsEnabled() && model.ProviderID+"/"+model.ID == req.Model {
			if req.Variant != "" {
				valid := false
				for _, variant := range model.Variants {
					valid = valid || variant.ID == req.Variant
				}
				if !valid {
					writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "selected variant is no longer available"})
					return
				}
			}
			if err := s.oc.SwitchModel(r.Context(), sessionID, opencode.ModelRef{ProviderID: model.ProviderID, ID: model.ID, Variant: req.Variant}); err != nil {
				writeChatUpstreamError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "selected model is no longer available"})
}

func (s *Server) chatCommand(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	var req struct {
		Command   string `json:"command"`
		Arguments string `json:"arguments"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	req.Command, req.Arguments = strings.TrimSpace(req.Command), strings.TrimSpace(req.Arguments)
	commands, err := s.oc.ListCommandsFor(r.Context(), s.st.Dir)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	for _, command := range commands {
		if command.Name == req.Command {
			if err := s.oc.RunCommand(r.Context(), sessionID, req.Command, req.Arguments); err != nil {
				writeChatUpstreamError(w, err)
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
			return
		}
	}
	writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "selected command is no longer available"})
}

func (s *Server) chatActivateSkill(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	var req struct {
		Skill string `json:"skill"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	req.Skill = strings.TrimSpace(req.Skill)
	ids, err := s.resolveChatSkills(r.Context(), []string{req.Skill})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	route, available, err := s.oc.StandaloneSkillRoute(r.Context())
	if err != nil || !available {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "standalone skill activation is unavailable; attach the skill to a prompt instead"})
		return
	}
	if err := s.oc.ActivateSkill(r.Context(), route, sessionID, ids[0]); err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) chatInterrupt(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if !s.chatMutationReady(w, sessionID, "", "") {
		return
	}
	interrupted, err := s.oc.Interrupt(r.Context(), sessionID)
	if err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"interrupted": interrupted})
}

func (s *Server) chatPermissionReply(w http.ResponseWriter, r *http.Request) {
	sessionID, requestID := r.PathValue("sessionID"), r.PathValue("requestID")
	if !s.chatMutationReady(w, sessionID, requestID, "per") {
		return
	}
	var req struct {
		Decision opencode.PermissionDecision `json:"decision"`
		Message  string                      `json:"message"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	if req.Decision != opencode.PermissionOnce && req.Decision != opencode.PermissionAlways && req.Decision != opencode.PermissionReject {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "decision must be once, always, or reject"})
		return
	}
	if err := s.oc.ReplyPermission(r.Context(), sessionID, requestID, req.Decision, strings.TrimSpace(req.Message)); err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) chatFormReply(w http.ResponseWriter, r *http.Request) {
	sessionID, formID := r.PathValue("sessionID"), r.PathValue("formID")
	if !s.chatMutationReady(w, sessionID, formID, "frm_") {
		return
	}
	var req struct {
		Answers opencode.FormAnswer `json:"answers"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	if req.Answers == nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "answers are required"})
		return
	}
	if err := s.oc.ReplyForm(r.Context(), sessionID, formID, req.Answers); err != nil {
		writeChatUpstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) chatMutationReady(w http.ResponseWriter, sessionID, resourceID, resourcePrefix string) bool {
	if !validChatID(sessionID, "ses_") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ses_ session id required"})
		return false
	}
	if resourcePrefix != "" && !validChatID(resourceID, resourcePrefix) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("valid %s id required", resourcePrefix)})
		return false
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return false
	}
	return true
}

func writeChatUpstreamError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *opencode.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
			status = apiErr.StatusCode
		}
	}
	message := "OpenCode could not complete the request"
	if status == http.StatusNotFound {
		message = "The requested OpenCode resource was not found"
	}
	if status == http.StatusConflict {
		message = "The session changed before the request could complete; refresh and retry"
	}
	if status == http.StatusUnprocessableEntity || status == http.StatusBadRequest {
		message = "OpenCode rejected the request; refresh the available options and retry"
	}
	writeJSON(w, status, map[string]string{"error": message})
}

// chatSession creates an unbound general-purpose coding session for the
// header Chat action. It shares the unassigned mapping bucket with discussions
// so the user can reopen it, but its prime forbids workflow bookkeeping.
func (s *Server) chatSession(w http.ResponseWriter, r *http.Request) {
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	title := "Codebase chat"
	sess, err := s.spawnSession(ctx, title)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	prime, modules := renderPrime("chat", primeContext{})
	if err := s.oc.Prompt(ctx, sess.ID, prime); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "prime chat session: " + err.Error()})
		return
	}
	entry := SessionEntry{
		Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339), Modules: modules,
	}
	if err := s.sessions.addUnassigned(entry); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("chat session created", "session", sess.ID)
	writeJSON(w, http.StatusCreated, sessionResponse{Session: entry.Session, Title: entry.Title, Created: entry.Created, Live: true})
}
