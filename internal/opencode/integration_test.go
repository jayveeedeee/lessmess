package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestIntegrationProjectionDropsSecretsAndUnknowns(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"hostile","name":"<img src=x onerror=alert(1)>","metadata":{"token":"metadata-canary"},"provider":{"settings":"provider-canary"},"methods":[{"id":"future","type":"future","label":"Future","command":["argv-canary"]},{"id":"oauth","type":"oauth","label":"OAuth","form":[{"key":"account","type":"string","title":"Account","default":"default-canary"},{"key":"bad","type":"external","url":"javascript:alert(1)"}]}],"connections":[{"type":"credential","id":"cred","label":"Work","secret":"credential-canary"},{"type":"env","name":"TOKEN","value":"env-canary"},{"type":"future","secret":"unknown-canary"}]}]}`))
	})
	items, err := c.ListIntegrations(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Methods[0].Type != "unknown" || items[0].Methods[1].Fields[1].URL != "" || items[0].Connections[2].Type != "unknown" {
		t.Fatalf("projection = %#v", items[0])
	}
	data, _ := json.Marshal(items)
	for _, forbidden := range []string{"metadata-canary", "provider-canary", "argv-canary", "default-canary", "credential-canary", "env-canary", "unknown-canary", "javascript:"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, data)
		}
	}
}

func TestAttemptStatusRedactsFailuresAndUnknownMessages(t *testing.T) {
	status := "failed"
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"status":"` + status + `","message":"raw-error-canary","time":{"created":1,"expires":2}}}`))
	})
	got, err := c.AttemptStatus(context.Background(), "/repo", "p", "oauth", "a")
	if err != nil || strings.Contains(got.Message, "canary") {
		t.Fatalf("status = %#v, err = %v", got, err)
	}
	status = "future"
	got, err = c.AttemptStatus(context.Background(), "/repo", "p", "command", "a")
	if err != nil || got.Status != "unknown" || got.Message != "" {
		t.Fatalf("unknown status = %#v, err = %v", got, err)
	}
}
