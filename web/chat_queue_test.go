package web

import (
	"os/exec"
	"testing"
)

// No Node build step or dependencies: these tests execute the actual vanilla
// client functions with a small DOM/fetch fixture. Go-only environments skip
// them; run node --test web/chat_queue_test.mjs explicitly for full verification.
func TestChatQueueClient(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable; run node --test web/chat_queue_test.mjs")
	}
	if out, err := exec.Command(node, "--test", "chat_queue_test.mjs").CombinedOutput(); err != nil {
		t.Fatalf("chat queue client regressions: %v\n%s", err, out)
	}
}
