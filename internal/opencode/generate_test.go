package opencode

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestGenerateTextContract(t *testing.T) {
	var body string
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Write([]byte(`{"paths":{"/api/experimental/generate":{"post":{}}}}`))
			return
		}
		if r.Method != "POST" || r.URL.Path != "/api/experimental/generate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body = readTestBody(r)
		w.Write([]byte(`{"data":{"text":"A descriptive title"}}`))
	})
	cap, err := c.LifecycleCapabilities(context.Background())
	if err != nil || !cap.Generate {
		t.Fatal(cap, err)
	}
	text, err := c.GenerateText(context.Background(), cap, "subject", nil)
	if err != nil || text != "A descriptive title" || body != `{"prompt":"subject"}` {
		t.Fatal(text, body, err)
	}
	_, err = c.GenerateText(context.Background(), cap, "subject", &ModelRef{ProviderID: "p", ID: "m"})
	if err != nil || !strings.Contains(body, `"providerID":"p"`) || !strings.Contains(body, `"id":"m"`) {
		t.Fatal(body, err)
	}
	if _, err := c.GenerateText(context.Background(), LifecycleCapabilities{}, "ignored", nil); !errors.Is(err, ErrCapabilityUnavailable) {
		t.Fatal(err)
	}
}
