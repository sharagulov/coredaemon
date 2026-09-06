package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the Ollama HTTP API.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// New creates a client for baseURL (e.g. http://localhost:11434) and model name.
func New(baseURL, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// chatOptions pin Ollama sampling: greedy decoding keeps tool-call JSON well-formed and
// makes the same question give the same answer, and num_ctx above the 4096 default keeps
// the system prompt from being shifted out once attachments and tool results pile up.
var chatOptions = map[string]any{
	"temperature": 0,
	"top_p":       0.9,
	"num_ctx":     8192,
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Tools    []Tool         `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options"`
}

type chatResponse struct {
	Message Message `json:"message"`
}

// ChatOnce sends one request to Ollama and returns the assistant message.
func (c *Client) ChatOnce(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	body, err := json.Marshal(chatRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
		Options:  chatOptions,
	})
	if err != nil {
		return Message{}, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("request ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Message{}, fmt.Errorf("ollama status %s: %s", resp.Status, bytes.TrimSpace(msg))
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Message{}, fmt.Errorf("decode response: %w", err)
	}
	return out.Message, nil
}
