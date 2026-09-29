package server

import (
	"context"
	"testing"
	"time"

	"lessmess/internal/docs"
)

// stubSeedSummarizer swaps the factory to capture agent/model and avoid LLM
// sessions. block, when non-nil, makes each SummarizeDir wait on it.
func stubSeedSummarizer(t *testing.T, block <-chan struct{}) (captured *struct{ agent, model string }) {
	t.Helper()
	captured = &struct{ agent, model string }{}
	orig := newSeedSummarizer
	t.Cleanup(func() { newSeedSummarizer = orig })
	newSeedSummarizer = func(_ docs.SessionClient, _ time.Duration, agent, model string) docs.Summarizer {
		captured.agent, captured.model = agent, model
		return &blockingSummarizer{block: block}
	}
	return captured
}

type blockingSummarizer struct{ block <-chan struct{} }

func (b *blockingSummarizer) SummarizeDir(ctx context.Context, _ string, _ *docs.Dir) error {
	if b.block != nil {
		select {
		case <-b.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestSessionDefaultsHelper(t *testing.T) {
	dir := t.TempDir()
	if a, m := SessionDefaults(dir); a != "" || m != "" {
		t.Errorf("empty repo defaults = %q %q, want service defaults", a, m)
	}
	if err := applySettingsPatch(dir, SettingsScopeProject, []byte(`{"session":{"agent":"build","model":"p/m"}}`)); err != nil {
		t.Fatal(err)
	}
	if a, m := SessionDefaults(dir); a != "build" || m != "p/m" {
		t.Errorf("project layer defaults = %q %q", a, m)
	}
	if err := applySettingsPatch(dir, SettingsScopePersonal, []byte(`{"session":{"agent":"plan"}}`)); err != nil {
		t.Fatal(err)
	}
	if a, m := SessionDefaults(dir); a != "plan" || m != "p/m" {
		t.Errorf("personal override = %q %q, want plan / p/m", a, m)
	}
}
