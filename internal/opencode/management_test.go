package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceIdentityCurrentAndBeta(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/info" {
				t.Fatalf("unexpected fallback %s", r.URL.Path)
			}
			w.Write([]byte(`{"version":"2.0.6","pid":123,"urls":["secret"],"paths":{"tmp":"/private/tmp/canary"}}`))
		})
		info, err := c.ServiceIdentity(context.Background())
		if err != nil || info.Version != "2.0.6" || info.Endpoint != "info" {
			t.Fatalf("info = %#v, %v", info, err)
		}
		data, _ := json.Marshal(info)
		if strings.Contains(string(data), "canary") || strings.Contains(string(data), "secret") || strings.Contains(string(data), "123") {
			t.Fatalf("unsafe projection: %s", data)
		}
	})
	t.Run("installed beta", func(t *testing.T) {
		c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/info" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.URL.Path != "/api/health" {
				t.Fatalf("path = %s", r.URL.Path)
			}
			w.Write([]byte(`{"healthy":true,"version":"0.0.0-beta-17519"}`))
		})
		info, err := c.ServiceIdentity(context.Background())
		if err != nil || info.Endpoint != "health_fallback" {
			t.Fatalf("info = %#v, %v", info, err)
		}
	})
}

func TestServiceIdentityDoesNotFallbackFromMalformedInfo(t *testing.T) {
	healthCalls := 0
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			healthCalls++
		}
		w.Write([]byte(`{"version":`))
	})
	if _, err := c.ServiceIdentity(context.Background()); err == nil || healthCalls != 0 {
		t.Fatalf("err = %v, health calls = %d", err, healthCalls)
	}
}

func TestManagementPositiveProjectionAndUnknownEnums(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/provider":
			w.Write([]byte(`{"data":[{"id":"p","name":"Provider","activation":"future","headers":{"Authorization":"canary"},"body":{"key":"secret"}}]}`))
		case "/api/plugin":
			w.Write([]byte(`{"data":[{"id":"plug","source":{"type":"local","path":"/private/canary"},"state":{"status":"failed","error":"secret raw error","ref":"/tmp/ref"}},{"id":"future","source":{"type":"future","path":"canary"},"state":{"status":"warming","error":"secret"}}]}`))
		}
	})
	providers, err := c.ListProvidersFor(context.Background(), "/repo")
	if err != nil || providers[0].Activation != "unknown" {
		t.Fatalf("providers = %#v, %v", providers, err)
	}
	plugins, err := c.ListPluginsFor(context.Background(), "/repo")
	if err != nil || plugins[0].Failure != "load_failed" || plugins[1].Status != "unknown" || plugins[1].SourceKind != "unknown" {
		t.Fatalf("plugins = %#v, %v", plugins, err)
	}
	data, _ := json.Marshal(struct{ Providers, Plugins any }{providers, plugins})
	for _, forbidden := range []string{"canary", "secret", "/private", "/tmp"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, data)
		}
	}
}

func TestManagementCapabilitiesExactMethodsAndPaths(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"paths":{"/api/info":{"get":{}},"/api/plugin":{"get":{}},"/api/plugin/update":{"post":{}},"/api/experimental/mcp/{server}/connect":{"post":{}},"/api/permission/saved":{"get":{},"post":{}}}}`))
	})
	cap, err := c.ManagementCapabilities(context.Background())
	if err != nil || !cap.ServiceInfo || !cap.Plugins || !cap.PluginUpdate || !cap.MCPConnect || !cap.SavedPermissions {
		t.Fatalf("cap = %#v, %v", cap, err)
	}
	if cap.HealthFallback || cap.MCPDisconnect || cap.RemovePermission || cap.ServiceRestart {
		t.Fatalf("false capability advertised: %#v", cap)
	}
}

func TestRediscoverRetargetsExistingClient(t *testing.T) {
	newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "opencode" || p != "newpw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"version":"2.0.6"}`))
	}))
	defer newServer.Close()
	oldDiscover, oldPassword := discoverService, readServicePassword
	discoverService = func(context.Context) (string, error) { return newServer.URL, nil }
	readServicePassword = func() (string, error) { return "newpw", nil }
	t.Cleanup(func() { discoverService, readServicePassword = oldDiscover, oldPassword })
	c := New("http://127.0.0.1:1", "oldpw")
	if err := c.Rediscover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.BaseURL() != newServer.URL {
		t.Fatalf("base = %q", c.BaseURL())
	}
	discoverService = func(context.Context) (string, error) { return "", errors.New("offline canary") }
	if err := c.Rediscover(context.Background()); err == nil || c.BaseURL() != newServer.URL {
		t.Fatalf("failed rediscovery changed client: %v", err)
	}
}
