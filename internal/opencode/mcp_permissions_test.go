package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestMCPCapabilityPathsCurrentAndInstalledBeta(t *testing.T) {
	for _, tc := range []struct {
		name, path string
	}{
		{"current experimental", "/api/experimental/mcp/{server}/connect"},
		{"installed beta", "/api/mcp/{server}/connect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requested string
			c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/openapi.json" {
					w.Write([]byte(`{"paths":{"` + tc.path + `":{"post":{}}}}`))
					return
				}
				requested = r.URL.RequestURI()
				w.WriteHeader(http.StatusNoContent)
			})
			cap, err := c.ManagementCapabilities(context.Background())
			if err != nil || !cap.MCPConnect || cap.MCPConnectPath != tc.path {
				t.Fatalf("cap = %#v, %v", cap, err)
			}
			if err := c.SetMCPConnected(context.Background(), cap, "/served repo", "hostile/name", true); err != nil {
				t.Fatal(err)
			}
			wantPath := strings.Replace(tc.path, "{server}", "hostile%2Fname", 1)
			if !strings.HasPrefix(requested, wantPath+"?") || !strings.Contains(requested, "location[directory]=%2Fserved+repo") {
				t.Fatalf("request = %q, want scoped %q", requested, wantPath)
			}
		})
	}
}

func TestMCPProjectionRedactsFailuresAndResourceURIs(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/mcp":
			w.Write([]byte(`{"data":[{"name":"<img onerror=alert(1)>","status":{"status":"failed","error":"Authorization: secret /private/key"}},{"name":"auth","status":{"status":"needs_auth","error":"https://user:pass@example.test/private?token=secret"},"integrationID":"provider"}]}`))
		case "/api/mcp/resource":
			w.Write([]byte(`{"data":{"resources":[{"server":"s","name":"r","uri":"https://user:pass@example.test/private/key?token=secret#frag"},{"server":"s","name":"f","uri":"file:///Users/person/.env"}],"templates":[{"server":"s","name":"t","uriTemplate":"https://example.test/private/{id}?token=secret"}]}}`))
		}
	})
	cap := ManagementCapabilities{MCP: true, MCPResources: true}
	servers, err := c.ListMCPFor(context.Background(), cap, "/repo")
	if err != nil || servers[0].Failure != "connection_failed" || servers[1].Failure != "authentication_required" {
		t.Fatalf("servers = %#v, %v", servers, err)
	}
	catalog, err := c.ListMCPResourcesFor(context.Background(), cap, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(struct{ Servers, Catalog any }{servers, catalog})
	for _, forbidden := range []string{"Authorization", "user:pass", "token=", "/private", "/Users", ".env", "secret", "#frag"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, data)
		}
	}
	if !strings.Contains(string(data), "example.test") || !strings.Contains(string(data), `\u003cimg`) {
		t.Fatalf("safe identity/hostile label missing: %s", data)
	}
}

func TestSavedPermissionsRequireExplicitProject(t *testing.T) {
	var query string
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{"data":[{"id":"rule","projectID":"project id","action":"read","resource":"/repo/**"}]}`))
	})
	cap := ManagementCapabilities{SavedPermissions: true}
	if _, err := c.ListSavedPermissions(context.Background(), cap, ""); err == nil {
		t.Fatal("empty project ID accepted")
	}
	rules, err := c.ListSavedPermissions(context.Background(), cap, "project id")
	if err != nil || len(rules) != 1 || query != "projectID=project+id" {
		t.Fatalf("rules/query = %#v %q, %v", rules, query, err)
	}
}
