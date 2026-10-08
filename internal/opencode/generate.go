package opencode

import (
	"context"
	"net/http"
)

// GenerateText is stateless and tool-free; it never adds a session message.
func (c *Client) GenerateText(ctx context.Context, cap LifecycleCapabilities, prompt string, model *ModelRef) (string, error) {
	if err := unavailable(cap.Generate, "stateless generation"); err != nil {
		return "", err
	}
	body := struct {
		Prompt string    `json:"prompt"`
		Model  *ModelRef `json:"model,omitempty"`
	}{prompt, model}
	var result struct {
		Text string `json:"text"`
	}
	err := c.do(ctx, http.MethodPost, "/api/experimental/generate", body, &result)
	return result.Text, err
}
