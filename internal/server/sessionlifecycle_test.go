package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLifecycleForkMapsChildWithParentScope(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/fork":{"post":{}}}}`))
		case "/api/session/ses_parent/fork":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["before"] != "msg_selected" {
				t.Errorf("fork body = %#v, err=%v", body, err)
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"data":{"id":"ses_child","title":"Child","parentID":"ses_parent","time":{"created":1000}}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_parent", Title: "Parent", Created: "x", Task: "MAC-10"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "POST", "/api/sessions/ses_parent/fork", `{"before":"msg_selected"}`)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":"ses_child"`) {
		t.Fatalf("fork = %d %s", w.Code, w.Body.String())
	}
	entries := s.sessions.list("2026-09-10-0")
	if len(entries) != 2 || entries[1].Session != "ses_child" || entries[1].Parent != "ses_parent" || entries[1].Task != "MAC-10" {
		t.Fatalf("child mapping = %#v", entries)
	}
}

func TestLifecycleRevertStaleThenCancelUsesCurrentFingerprint(t *testing.T) {
	cleared := 0
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/revert":{"delete":{}}}}`))
		case "/api/session/ses_life":
			w.Write([]byte(`{"data":{"id":"ses_life","revert":{"messageID":"msg_1","files":[{"file":"a.go","patch":"@@ -1 +1 @@","additions":1,"deletions":1}]},"time":{"created":1,"updated":20}}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		case "/api/session/ses_life/revert":
			cleared++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	stale := do(t, s.Handler(), "POST", "/api/sessions/ses_life/revert/clear", `{"confirmation":{"sessionID":"ses_life","updated":19,"messageID":"msg_1"}}`)
	if stale.Code != http.StatusConflict || cleared != 0 || !strings.Contains(stale.Body.String(), "refresh and retry") {
		t.Fatalf("stale clear = %d called=%d %s", stale.Code, cleared, stale.Body.String())
	}
	valid := do(t, s.Handler(), "POST", "/api/sessions/ses_life/revert/clear", `{"confirmation":{"sessionID":"ses_life","updated":20,"messageID":"msg_1"}}`)
	if valid.Code != http.StatusNoContent || cleared != 1 {
		t.Fatalf("valid clear = %d called=%d %s", valid.Code, cleared, valid.Body.String())
	}
}

func TestLifecycleCompactBusyDoesNotQueue(t *testing.T) {
	queued := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_life":
			w.Write([]byte(`{"data":{"id":"ses_life","time":{"created":1,"updated":20}}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{"ses_life":{"type":"running"}}}`))
		case "/api/session/ses_life/compact":
			queued = true
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	w := do(t, s.Handler(), "POST", "/api/sessions/ses_life/compact", `{"delivery":"queue","confirmation":{"sessionID":"ses_life","updated":20}}`)
	if w.Code != http.StatusConflict || queued || !strings.Contains(w.Body.String(), "busy") {
		t.Fatalf("compact busy = %d queued=%v %s", w.Code, queued, w.Body.String())
	}
}

func TestLifecycleConfirmationStaleBusyMissingAndNoLeak(t *testing.T) {
	busy := false
	missing := false
	mutated := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/revert/stage":{"post":{}}}}`))
		case "/api/session/ses_life":
			if missing {
				w.WriteHeader(404)
				w.Write([]byte(`{"message":"raw secret missing"}`))
				return
			}
			w.Write([]byte(`{"data":{"id":"ses_life","title":"safe","time":{"created":1,"updated":20},"location":{"directory":"/private/repo"}}}`))
		case "/api/session/active":
			if busy {
				w.Write([]byte(`{"data":{"ses_life":{"type":"running"}}}`))
			} else {
				w.Write([]byte(`{"data":{}}`))
			}
		case "/api/session/ses_life/revert/stage":
			mutated = true
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	cases := []struct {
		body string
		code int
	}{
		{`{"messageID":"msg_1","files":true}`, http.StatusPreconditionRequired},
		{`{"messageID":"bad","files":true,"confirmation":{"sessionID":"ses_life","updated":20}}`, http.StatusBadRequest},
		{`{"messageID":"msg_1","files":true,"confirmation":{"sessionID":"ses_other","updated":20}}`, http.StatusConflict},
		{`{"messageID":"msg_1","files":true,"confirmation":{"sessionID":"ses_life","updated":19}}`, http.StatusConflict},
	}
	for _, tc := range cases {
		w := do(t, s.Handler(), "POST", "/api/sessions/ses_life/revert/stage", tc.body)
		if w.Code != tc.code {
			t.Errorf("body %s: %d %s", tc.body, w.Code, w.Body.String())
		}
	}
	busy = true
	w := do(t, s.Handler(), "POST", "/api/sessions/ses_life/revert/stage", `{"messageID":"msg_1","files":false,"confirmation":{"sessionID":"ses_life","updated":20}}`)
	if w.Code != http.StatusConflict || mutated {
		t.Fatalf("busy = %d mutated=%v body=%s", w.Code, mutated, w.Body.String())
	}
	busy = false
	missing = true
	w = do(t, s.Handler(), "POST", "/api/sessions/ses_life/revert/stage", `{"messageID":"msg_1","files":false,"confirmation":{"sessionID":"ses_life","updated":20}}`)
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "raw secret") {
		t.Fatalf("missing = %d %s", w.Code, w.Body.String())
	}
}

func TestLifecyclePublishedRoutesPreviewDeliveryRenameAndUnavailableExport(t *testing.T) {
	seen := map[string]bool{}
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.RequestURI()] = true
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}":{"patch":{}},"/api/session/{sessionID}/prompt":{"post":{"requestBody":{"content":{"application/json":{"schema":{"properties":{"id":{},"text":{},"files":{},"skills":{},"delivery":{}}}}}}}},"/api/session/{sessionID}/inbox":{"get":{}},"/api/session/{sessionID}/inbox/{inboxID}":{"patch":{},"delete":{}}}}`))
		case "/api/session/ses_life":
			if r.Method == http.MethodPatch {
				w.WriteHeader(204)
				return
			}
			w.Write([]byte(`{"data":{"id":"ses_life","title":"safe","revert":{"messageID":"msg_1","snapshot":"must-not-leak","files":[{"file":"a.go","patch":"safe","additions":1}]},"time":{"created":1,"updated":20},"location":{"directory":"/private/repo"}}}`))
		case "/api/session/ses_life/prompt":
			w.Write([]byte(`{"data":{"id":"msg_in","sessionID":"ses_life","type":"user","delivery":"queue","payload":{"text":"next"},"time":{"created":2}}}`))
		case "/api/session/ses_life/inbox":
			w.Write([]byte(`{"data":[{"id":"msg_x","sessionID":"ses_life","type":"future","delivery":"queue","payload":{"token":"raw-secret"}}]}`))
		default:
			w.WriteHeader(204)
		}
	})
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_life", Title: "old fallback", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_life/revert/preview", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"file":"a.go"`) || strings.Contains(w.Body.String(), "snapshot") {
		t.Fatalf("preview = %d %s", w.Code, w.Body.String())
	}
	w = do(t, s.Handler(), "POST", "/api/sessions/ses_life/deliver", `{"id":"msg_stable","text":"next","delivery":"queue"}`)
	if w.Code != 202 || !strings.Contains(w.Body.String(), `"id":"msg_in"`) {
		t.Fatalf("deliver = %d %s", w.Code, w.Body.String())
	}
	w = do(t, s.Handler(), "GET", "/api/sessions/ses_life/inbox", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "raw-secret") {
		t.Fatalf("inbox = %d %s", w.Code, w.Body.String())
	}
	w = do(t, s.Handler(), "PATCH", "/api/sessions/ses_life", `{"title":"Renamed"}`)
	if w.Code != 204 || !seen["PATCH /api/session/ses_life"] {
		t.Fatalf("rename = %d %s seen=%v", w.Code, w.Body.String(), seen)
	}
	if got := s.sessions.listUnassigned()[0].Title; got != "Renamed" {
		t.Fatalf("persisted fallback title = %q", got)
	}
	w = do(t, s.Handler(), "GET", "/api/sessions/ses_life/export", "")
	if w.Code != 503 || strings.Contains(w.Body.String(), "openapi") {
		t.Fatalf("export unavailable = %d %s", w.Code, w.Body.String())
	}
}

func TestLifecycleMalformedIDsBodiesAndUnknownQueries(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"paths":{}}`)) })
	cases := []struct {
		method, path, body string
		code               int
	}{
		{"POST", "/api/sessions/bad/fork", `{"before":"msg_1"}`, 400},
		{"POST", "/api/sessions/ses_x/fork", `{"before":"bad"}`, 400},
		{"POST", "/api/sessions/ses_x/fork", `{"before":"msg_1","extra":1}`, 400},
		{"PATCH", "/api/sessions/ses_x/inbox/bad", `{"delivery":"queue"}`, 400},
		{"PATCH", "/api/sessions/ses_x/inbox/msg_1", `{"delivery":"later"}`, 422},
		{"GET", "/api/sessions/ses_x/children?wat=1", "", 400},
		{"GET", "/api/sessions/ses_x/children?limit=101", "", 400},
	}
	for _, tc := range cases {
		w := do(t, s.Handler(), tc.method, tc.path, tc.body)
		if w.Code != tc.code {
			t.Errorf("%s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestInboxOrderingUnionRedactionAndBusyCancellation(t *testing.T) {
	var deliveredIDs []string
	cancelled := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/prompt":{"post":{"requestBody":{"content":{"application/json":{"schema":{"properties":{"id":{},"text":{},"files":{},"skills":{},"delivery":{}}}}}}}},"/api/session/{sessionID}/inbox":{"get":{}},"/api/session/{sessionID}/inbox/{inboxID}":{"patch":{},"delete":{}}}}`))
		case "/api/session/ses_life/prompt":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			deliveredIDs = append(deliveredIDs, body["id"].(string))
			w.Write([]byte(`{"data":{"id":"msg_stable","sessionID":"ses_life","type":"user","delivery":"queue","payload":{"text":"next"},"time":{"created":5}}}`))
		case "/api/session/ses_life/inbox":
			w.Write([]byte(`{"data":[{"id":"msg_move","sessionID":"ses_life","type":"move","delivery":"queue","payload":{"projectID":"secret-project","location":{"directory":"/secret"}},"time":{"created":4}},{"id":"msg_user","sessionID":"ses_life","type":"user","delivery":"steer","payload":{"text":"visible","files":[{"uri":"data:text/plain;base64,c2VjcmV0"}]},"time":{"created":1}},{"id":"msg_unknown","sessionID":"ses_life","type":"future","delivery":"queue","payload":{"token":"raw-secret"},"time":{"created":3}},{"id":"msg_compact","sessionID":"ses_life","type":"compaction","delivery":"queue","payload":{"summary":"private"},"time":{"created":2}},{"id":"msg_wrong","sessionID":"ses_other","type":"user","delivery":"queue","payload":{"text":"wrong session"},"time":{"created":0}},{"id":"msg_synth","sessionID":"ses_life","type":"synthetic","delivery":"queue","payload":{"text":"generated","description":"safe description","metadata":{"token":"private"}},"time":{"created":2}}]}`))
		case "/api/session/ses_life/inbox/msg_user":
			if r.Method != http.MethodDelete {
				t.Errorf("cancel method = %s", r.Method)
			}
			cancelled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	for range 2 {
		w := do(t, s.Handler(), "POST", "/api/sessions/ses_life/deliver", `{"id":"msg_stable","text":"next","delivery":"queue"}`)
		if w.Code != http.StatusAccepted {
			t.Fatalf("deliver = %d %s", w.Code, w.Body.String())
		}
	}
	if len(deliveredIDs) != 2 || deliveredIDs[0] != deliveredIDs[1] {
		t.Fatalf("stable delivery IDs = %#v", deliveredIDs)
	}

	w := do(t, s.Handler(), "GET", "/api/sessions/ses_life/inbox", "")
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("inbox = %d %s", w.Code, body)
	}
	order := []string{`"id":"msg_user"`, `"id":"msg_compact"`, `"id":"msg_synth"`, `"id":"msg_unknown"`, `"id":"msg_move"`}
	last := -1
	for _, marker := range order {
		at := strings.Index(body, marker)
		if at <= last {
			t.Fatalf("inbox order missing %s: %s", marker, body)
		}
		last = at
	}
	for _, secret := range []string{"raw-secret", "secret-project", "/secret", "private", "wrong session", "base64"} {
		if strings.Contains(body, secret) {
			t.Fatalf("inbox leaked %q: %s", secret, body)
		}
	}
	for _, visible := range []string{`"text":"visible"`, `"text":"generated"`, `"description":"safe description"`, `"known":false`} {
		if !strings.Contains(body, visible) {
			t.Errorf("inbox missing %s: %s", visible, body)
		}
	}

	// Cancellation is deliberately best-effort and does not require the session
	// to be idle; a 204 only means the item is no longer cancellable.
	w = do(t, s.Handler(), "DELETE", "/api/sessions/ses_life/inbox/msg_user", "")
	if w.Code != http.StatusNoContent || !cancelled {
		t.Fatalf("cancel = %d called=%v body=%s", w.Code, cancelled, w.Body.String())
	}
}

func TestLifecycleDeleteCleansParentAndDescendantMappings(t *testing.T) {
	deleted := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}":{"delete":{}}}}`))
		case "/api/session/ses_parent":
			if r.Method == http.MethodDelete {
				deleted = true
				w.WriteHeader(204)
				return
			}
			w.Write([]byte(`{"data":{"id":"ses_parent","title":"Parent <unsafe>","time":{"created":1,"updated":20},"location":{"directory":"/repo"}}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		case "/api/session":
			switch r.URL.Query().Get("parentID") {
			case "ses_parent":
				w.Write([]byte(`{"data":[{"id":"ses_child","parentID":"ses_parent","time":{"created":2,"updated":3},"location":{"directory":"/repo"}}],"cursor":{}}`))
			case "ses_child":
				w.Write([]byte(`{"data":[{"id":"ses_grand","parentID":"ses_child","time":{"created":3,"updated":4},"location":{"directory":"/repo"}}],"cursor":{}}`))
			default:
				w.Write([]byte(`{"data":[],"cursor":{}}`))
			}
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.RequestURI())
		}
	})
	for _, id := range []string{"ses_parent", "ses_child", "ses_grand"} {
		if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: id, Title: id, Created: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.sessions.remove("2026-09-10-0", "ses_grand"); err != nil {
		t.Fatal(err)
	}
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_grand", Title: "grand fallback", Created: "x", Parent: "ses_child"}); err != nil {
		t.Fatal(err)
	}
	previewResponse := do(t, s.Handler(), "GET", "/api/sessions/ses_parent/delete-preview", "")
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("preview = %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview sessionDeletePreview
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Count != 3 || len(preview.Mappings) != 3 || preview.Title != "Parent <unsafe>" {
		t.Fatalf("preview = %#v", preview)
	}
	body, _ := json.Marshal(map[string]any{"confirmation": map[string]any{"sessionID": preview.SessionID, "fingerprint": preview.Fingerprint, "count": preview.Count, "title": preview.Title}})
	w := do(t, s.Handler(), "DELETE", "/api/sessions/ses_parent", string(body))
	if w.Code != http.StatusNoContent || !deleted {
		t.Fatalf("delete = %d %s, called=%v", w.Code, w.Body.String(), deleted)
	}
	if got := s.sessions.list("2026-09-10-0"); len(got) != 0 {
		t.Fatalf("stale mappings = %#v", got)
	}
	if got := s.sessions.listUnassigned(); len(got) != 0 {
		t.Fatalf("stale unassigned mappings = %#v", got)
	}
}

func TestLifecycleDeleteRejectsStaleTreeConfirmation(t *testing.T) {
	title := "Parent"
	deleted := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}":{"delete":{}}}}`))
		case "/api/session/ses_parent":
			if r.Method == http.MethodDelete {
				deleted = true
				w.WriteHeader(204)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ses_parent", "title": title, "time": map[string]int64{"updated": 1}}})
		case "/api/session":
			w.Write([]byte(`{"data":[],"cursor":{}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_parent/delete-preview", "")
	var preview sessionDeletePreview
	json.Unmarshal(w.Body.Bytes(), &preview)
	title = "Changed"
	body, _ := json.Marshal(map[string]any{"confirmation": map[string]any{"sessionID": preview.SessionID, "fingerprint": preview.Fingerprint, "count": preview.Count, "title": preview.Title}})
	w = do(t, s.Handler(), "DELETE", "/api/sessions/ses_parent", string(body))
	if w.Code != http.StatusConflict || deleted || !strings.Contains(w.Body.String(), "refresh") {
		t.Fatalf("stale delete = %d deleted=%v %s", w.Code, deleted, w.Body.String())
	}
}

func TestLifecycleDeleteMappingFailureCanBeRetried(t *testing.T) {
	deleted := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}":{"delete":{}}}}`))
		case "/api/session/ses_retry":
			if r.Method == http.MethodDelete {
				deleted = true
				w.WriteHeader(204)
				return
			}
			if deleted {
				w.WriteHeader(404)
				w.Write([]byte(`{"message":"gone"}`))
				return
			}
			w.Write([]byte(`{"data":{"id":"ses_retry","title":"Retry","time":{"updated":1}}}`))
		case "/api/session":
			w.Write([]byte(`{"data":[],"cursor":{}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		}
	})
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_retry", Title: "Retry", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_retry/delete-preview", "")
	var preview sessionDeletePreview
	json.Unmarshal(w.Body.Bytes(), &preview)
	body, _ := json.Marshal(map[string]any{"confirmation": map[string]any{"sessionID": preview.SessionID, "fingerprint": preview.Fingerprint, "count": preview.Count, "title": preview.Title}})
	originalPath := s.sessions.path
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.sessions.path = filepath.Join(blocker, "sessions.json")
	w = do(t, s.Handler(), "DELETE", "/api/sessions/ses_retry", string(body))
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"recoverable":true`) || len(s.sessions.list("2026-09-10-0")) != 1 {
		t.Fatalf("partial delete = %d %s mappings=%#v", w.Code, w.Body.String(), s.sessions.list("2026-09-10-0"))
	}
	s.sessions.path = originalPath
	w = do(t, s.Handler(), "DELETE", "/api/sessions/ses_retry", string(body))
	if w.Code != http.StatusNoContent || len(s.sessions.list("2026-09-10-0")) != 0 {
		t.Fatalf("recovery delete = %d %s mappings=%#v", w.Code, w.Body.String(), s.sessions.list("2026-09-10-0"))
	}
}

func TestSessionExportHeadersErrorsAndUnlinkIdempotence(t *testing.T) {
	if got := safeSessionExportFilename("ses_bad\r\n\"/../../x"); got != "session-ses_bad__________x.json" {
		t.Fatalf("safe export filename = %q", got)
	}
	exportFails := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/experimental/session/{sessionID}/export":{"get":{}}}}`))
		case "/api/experimental/session/ses_safe/export":
			if r.URL.Query().Get("sanitize") != "true" {
				t.Error("export was not sanitized")
			}
			if exportFails {
				w.WriteHeader(502)
				w.Write([]byte(`{"message":"secret upstream"}`))
				return
			}
			w.Write([]byte(`{"data":{"info":{"id":"ses_safe","title":"hostile\" title"},"messages":[]}}`))
		}
	})
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_safe", Title: "fallback", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_safe/export", "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Disposition") != `attachment; filename="session-ses_safe.json"` {
		t.Fatalf("export = %d headers=%v %s", w.Code, w.Header(), w.Body.String())
	}
	exportFails = true
	w = do(t, s.Handler(), "GET", "/api/sessions/ses_safe/export", "")
	if w.Code != http.StatusBadGateway || w.Header().Get("Content-Disposition") != "" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || strings.Contains(w.Body.String(), "secret upstream") {
		t.Fatalf("export error = %d headers=%v %s", w.Code, w.Header(), w.Body.String())
	}
	for range 2 {
		w = do(t, s.Handler(), "DELETE", "/api/sessions/ses_safe/mapping", "")
		if w.Code != http.StatusNoContent {
			t.Fatalf("unlink = %d %s", w.Code, w.Body.String())
		}
	}
}

func TestSessionNavigationAncestorsOwnershipPendingAndRefresh(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_child":
			w.Write([]byte(`{"data":{"id":"ses_child","title":"Child","parentID":"ses_parent","time":{"created":2,"updated":3}}}`))
		case "/api/session/ses_parent":
			w.Write([]byte(`{"data":{"id":"ses_parent","title":"Parent","time":{"created":1,"updated":4}}}`))
		case "/api/session":
			if r.URL.Query().Get("parentID") == "ses_child" {
				w.Write([]byte(`{"data":[{"id":"ses_grand","title":"FIX-00: Grand","parentID":"ses_child","time":{"created":3,"updated":5}}],"cursor":{}}`))
			} else {
				w.Write([]byte(`{"data":[],"cursor":{}}`))
			}
		case "/api/session/active":
			w.Write([]byte(`{"data":{"ses_grand":{"type":"running"}}}`))
		case "/api/session/ses_child/form":
			w.Write([]byte(`{"data":[{"id":"frm_1","sessionID":"ses_child","title":"Answer","fields":[]}]}`))
		case "/api/session/ses_grand/permission":
			w.Write([]byte(`{"data":[{"id":"per_1","sessionID":"ses_grand","action":"write"}]}`))
		default:
			if strings.HasSuffix(r.URL.Path, "/permission") || strings.HasSuffix(r.URL.Path, "/form") {
				w.Write([]byte(`{"data":[]}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_parent", Title: "Parent", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_child", Title: "Child", Created: "x", Task: "FIX-01", Parent: "ses_parent"}); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		w := do(t, s.Handler(), "GET", "/api/sessions/ses_child/navigation", "")
		if w.Code != http.StatusOK {
			t.Fatalf("navigation = %d %s", w.Code, w.Body.String())
		}
		var response struct {
			Current     sessionNavigationItem   `json:"current"`
			Ancestors   []sessionNavigationItem `json:"ancestors"`
			Descendants []sessionNavigationItem `json:"descendants"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Current.Task != "FIX-01" || response.Current.Change != "2026-09-10-0" || response.Current.PendingInput != 1 {
			t.Fatalf("current = %#v", response.Current)
		}
		if len(response.Ancestors) != 1 || response.Ancestors[0].Session != "ses_parent" {
			t.Fatalf("ancestors = %#v", response.Ancestors)
		}
		if len(response.Descendants) != 1 || response.Descendants[0].Session != "ses_grand" || !response.Descendants[0].Busy || response.Descendants[0].PendingInput != 1 || response.Descendants[0].Task != "FIX-00" {
			t.Fatalf("descendants = %#v", response.Descendants)
		}
	}
	entries := s.sessions.list("2026-09-10-0")
	if len(entries) != 3 {
		t.Fatalf("refresh duplicated mappings: %#v", entries)
	}
}
