package docs

import (
	"context"
	"testing"
	"time"

	"lessmess/internal/opencode"
)

// captureClient records the agent/model passed at session creation.
type captureClient struct {
	agent string
	model *opencode.ModelRef
}

func (c *captureClient) CreateSession(context.Context, string, string) (*opencode.Session, error) {
	return &opencode.Session{ID: "s"}, nil
}

func (c *captureClient) CreateSessionWith(_ context.Context, _, _, agent string, model *opencode.ModelRef) (*opencode.Session, error) {
	c.agent, c.model = agent, model
	return &opencode.Session{ID: "s"}, nil
}

func (c *captureClient) Prompt(context.Context, string, string) error { return nil }
func (c *captureClient) WaitDone(context.Context, string) error       { return nil }
func (c *captureClient) DeleteSession(context.Context, string) error  { return nil }

func TestOpenCodeSummarizerHonorsDefaults(t *testing.T) {
	cc := &captureClient{}
	sum := NewOpenCodeSummarizerWith(cc, time.Second, "build", "prov/some/model")
	if err := sum.SummarizeDir(context.Background(), "/repo", &Dir{Rel: "internal/x"}); err != nil {
		t.Fatal(err)
	}
	if cc.agent != "build" {
		t.Errorf("agent = %q, want build", cc.agent)
	}
	// Model IDs may contain slashes: split on the FIRST slash only.
	if cc.model == nil || cc.model.ProviderID != "prov" || cc.model.ID != "some/model" {
		t.Errorf("model = %+v, want prov / some/model", cc.model)
	}
}

func TestOpenCodeSummarizerServiceDefaults(t *testing.T) {
	cc := &captureClient{}
	sum := NewOpenCodeSummarizer(cc, 0)
	if err := sum.SummarizeDir(context.Background(), "/repo", &Dir{Rel: "x"}); err != nil {
		t.Fatal(err)
	}
	if cc.agent != "" || cc.model != nil {
		t.Errorf("default constructor must leave service defaults: agent=%q model=%+v", cc.agent, cc.model)
	}
}
