package web

import (
	"os/exec"
	"testing"
)

func TestChatsClient(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable; run node --test web/chats_test.mjs")
	}
	if out, err := exec.Command(node, "--test", "chats_test.mjs").CombinedOutput(); err != nil {
		t.Fatalf("Chats client regressions: %v\n%s", err, out)
	}
}
