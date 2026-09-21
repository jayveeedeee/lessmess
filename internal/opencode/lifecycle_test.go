package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestLifecyclePublishedContracts(t *testing.T) {
	seen := map[string]string{}
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.RequestURI()] = readTestBody(r)
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/fork":{"post":{"requestBody":{"content":{"application/json":{"schema":{"properties":{"before":{"type":"string"}}}}}}}},"/api/session/{sessionID}/revert/stage":{"post":{}},"/api/session/{sessionID}/revert/commit":{"post":{}},"/api/session/{sessionID}/revert":{"delete":{}},"/api/session/{sessionID}/compact":{"post":{}},"/api/session/{sessionID}/prompt":{"post":{"requestBody":{"content":{"application/json":{"schema":{"properties":{"id":{},"text":{},"files":{},"skills":{},"delivery":{}}}}}}}},"/api/session/{sessionID}/inbox":{"get":{}},"/api/session/{sessionID}/inbox/{inboxID}":{"patch":{},"delete":{}},"/api/session/{sessionID}":{"patch":{},"delete":{}},"/api/experimental/session/{sessionID}/export":{"get":{}}}}`))
		case "/api/session/ses_1/fork":
			w.Write([]byte(`{"data":{"id":"ses_child","time":{"created":1,"updated":2},"location":{"directory":"/repo"}}}`))
		case "/api/session/ses_1/prompt":
			w.Write([]byte(`{"data":{"id":"msg_in","sessionID":"ses_1","type":"user","delivery":"steer","payload":{"text":"go"},"time":{"created":3}}}`))
		case "/api/session/ses_1/inbox":
			w.Write([]byte(`{"data":[{"id":"msg_in","sessionID":"ses_1","type":"future","delivery":"queue","payload":{"text":"must not survive","secret":"must not survive"}}]}`))
		case "/api/experimental/session/ses_1/export":
			w.Write([]byte(`{"data":{"info":{"id":"ses_1","time":{"created":1,"updated":2},"location":{"directory":"/repo"}},"messages":[]}}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	ctx := context.Background()
	cap, err := c.LifecycleCapabilities(ctx)
	if err != nil || !cap.RevertClear || cap.RevertClearMethod != http.MethodDelete || !cap.InboxDelivery || cap.InboxDeliveryMethod != http.MethodPatch || !cap.Export || !cap.Rename {
		t.Fatalf("capabilities = %#v, %v", cap, err)
	}
	if _, err := c.ForkSession(ctx, cap, "ses_1", "msg_1"); err != nil {
		t.Fatal(err)
	}
	item, err := c.DeliverPrompt(ctx, cap, "ses_1", "msg_stable", "go", []PromptFile{{URI: "file:///repo/a.go", Name: "a.go"}}, []string{"review"}, DeliverySteer)
	if err != nil || item.ID != "msg_in" || !item.Known {
		t.Fatalf("delivery = %#v, %v", item, err)
	}
	items, err := c.ListInbox(ctx, cap, "ses_1")
	if err != nil || len(items) != 1 || items[0].Known || items[0].Text != "" || items[0].Description != "" {
		t.Fatalf("unknown inbox = %#v, %v", items, err)
	}
	if err := c.ChangeInboxDelivery(ctx, cap, "ses_1", "msg_in", DeliveryQueue); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearRevert(ctx, cap, "ses_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExportSessionSanitized(ctx, cap, "ses_1"); err != nil {
		t.Fatal(err)
	}
	if got := seen["POST /api/session/ses_1/fork"]; !strings.Contains(got, `"before":"msg_1"`) {
		t.Errorf("fork body = %s", got)
	}
	if got := seen["PATCH /api/session/ses_1/inbox/msg_in"]; got != `{"delivery":"queue"}` {
		t.Errorf("delivery body = %s", got)
	}
	if got := seen["POST /api/session/ses_1/prompt"]; !strings.Contains(got, `"id":"msg_stable"`) || !strings.Contains(got, `"files":[`) || !strings.Contains(got, `"skills":[{"id":"review"}]`) {
		t.Errorf("rich delivery body = %s", got)
	}
	if _, ok := seen["DELETE /api/session/ses_1/revert"]; !ok {
		t.Error("published revert clear not used")
	}
	if _, ok := seen["GET /api/experimental/session/ses_1/export?sanitize=true"]; !ok {
		t.Error("sanitized export not used")
	}
}

func TestLifecycleInstalledVariants(t *testing.T) {
	seen := map[string]string{}
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path] = readTestBody(r)
		if r.URL.Path == "/openapi.json" {
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/fork":{"post":{"requestBody":{"content":{"application/json":{"schema":{"required":["boundary"],"properties":{"boundary":{}}}}}}}},"/api/session/{sessionID}/revert/clear":{"post":{}},"/api/session/{sessionID}/inbox/{inboxID}/queue":{"post":{}},"/api/session/{sessionID}/inbox/{inboxID}/steer":{"post":{}},"/api/session/{sessionID}/rename":{"post":{}}}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/fork") {
			w.Write([]byte(`{"data":{"id":"ses_child","time":{},"location":{}}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	cap, err := c.LifecycleCapabilities(context.Background())
	if err != nil || !cap.ForkBoundary || cap.RevertClearMethod != http.MethodPost || cap.InboxDeliveryPath == "" || cap.Export || !cap.Rename {
		t.Fatalf("cap = %#v, %v", cap, err)
	}
	if _, err := c.ForkSession(context.Background(), cap, "ses_1", "msg_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearRevert(context.Background(), cap, "ses_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.ChangeInboxDelivery(context.Background(), cap, "ses_1", "msg_1", DeliverySteer); err != nil {
		t.Fatal(err)
	}
	if err := c.RenameSessionCompatible(context.Background(), cap, "ses_1", "new"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen["POST /api/session/ses_1/fork"], `"boundary":{"messageID":"msg_1","type":"before"}`) {
		t.Errorf("fork = %s", seen["POST /api/session/ses_1/fork"])
	}
	if _, ok := seen["POST /api/session/ses_1/revert/clear"]; !ok {
		t.Error("installed clear not used")
	}
	if _, ok := seen["POST /api/session/ses_1/inbox/msg_1/steer"]; !ok {
		t.Error("installed steer not used")
	}
	if _, ok := seen["POST /api/session/ses_1/rename"]; !ok {
		t.Error("installed rename not used")
	}
	if _, err := c.ExportSessionSanitized(context.Background(), cap, "ses_1"); !errorsIsCapability(err) {
		t.Fatalf("export error = %v", err)
	}
}

func TestMessageDecodesShellAndCompactionWithoutUnknownFailure(t *testing.T) {
	var messages []Message
	if err := json.Unmarshal([]byte(`[{"id":"msg_s","type":"shell","shellID":"sh_1","command":"pwd","status":"exited","exit":0,"output":{"output":"/secret/path","cursor":1,"size":12,"truncated":true},"time":{"created":1}},{"id":"msg_c","type":"compaction","status":"future","reason":"manual","time":{"created":2}}]`), &messages); err != nil {
		t.Fatal(err)
	}
	if messages[0].ShellID != "sh_1" || messages[0].Output == nil || !messages[0].Output.Truncated || messages[1].Type != "compaction" {
		t.Fatalf("messages = %#v", messages)
	}
}

func readTestBody(r *http.Request) string {
	var value json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&value)
	return string(value)
}
func errorsIsCapability(err error) bool { return errors.Is(err, ErrCapabilityUnavailable) }
