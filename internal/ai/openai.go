package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAIURL = "https://api.openai.com/v1"

// OpenAIClient talks to the OpenAI Chat Completions API (or a compatible /v1 host).
type OpenAIClient struct {
	baseURL string
	model   string
	key     string
	http    *http.Client
}

// NewOpenAI creates a client for baseURL (e.g. https://api.openai.com/v1), model, and API key.
func NewOpenAI(baseURL, model, key string) *OpenAIClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenAIURL
	}
	return &OpenAIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		key:     strings.TrimSpace(key),
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *OpenAIClient) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []Tool          `json:"tools,omitempty"`
	Temperature float64         `json:"temperature"`
}

type openAIMessage struct {
	Role       string       `json:"role"`
	Content    *string      `json:"content,omitempty"`
	ToolCalls  []openAICall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type openAICall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function openAICallFn `json:"function"`
}

type openAICallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIWireMessage `json:"message"`
	} `json:"choices"`
}

type openAIWireMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolCalls []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

// ChatOnce sends one request to OpenAI and returns the assistant message in the agent shape.
func (c *OpenAIClient) ChatOnce(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if c == nil || c.key == "" {
		return Message{}, ErrOpenAIUnavailable
	}

	body, err := json.Marshal(openAIRequest{
		Model:       c.model,
		Messages:    encodeOpenAIMessages(messages),
		Tools:       tools,
		Temperature: 0,
	})
	if err != nil {
		return Message{}, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("request openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Message{}, fmt.Errorf("openai status %s: %s", resp.Status, bytes.TrimSpace(msg))
	}

	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Message{}, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Choices) == 0 {
		return Message{}, errors.New("empty response from openai")
	}
	return decodeOpenAIMessage(out.Choices[0].Message), nil
}

func encodeOpenAIMessages(msgs []Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(msgs))
	pending := make([]string, 0)
	seq := 0
	nextID := func(existing string) string {
		if existing != "" {
			return existing
		}
		seq++
		return fmt.Sprintf("call_%d", seq)
	}

	for _, m := range msgs {
		switch m.Role {
		case RoleAssistant:
			pending = pending[:0]
			om := openAIMessage{Role: RoleAssistant}
			if m.Content != "" {
				c := m.Content
				om.Content = &c
			}
			for _, tc := range m.ToolCalls {
				id := nextID(tc.ID)
				pending = append(pending, id)
				typ := tc.Type
				if typ == "" {
					typ = "function"
				}
				om.ToolCalls = append(om.ToolCalls, openAICall{
					ID:   id,
					Type: typ,
					Function: openAICallFn{
						Name:      tc.Function.Name,
						Arguments: openAIArgsString(tc.Function.Arguments),
					},
				})
			}
			out = append(out, om)
		case RoleTool:
			id := ""
			if len(pending) > 0 {
				id = pending[0]
				pending = pending[1:]
			} else {
				id = nextID("")
			}
			c := m.Content
			out = append(out, openAIMessage{Role: RoleTool, Content: &c, ToolCallID: id})
		default:
			c := m.Content
			out = append(out, openAIMessage{Role: m.Role, Content: &c})
		}
	}
	return out
}

func openAIArgsString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "{}"
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if strings.TrimSpace(s) == "" {
				return "{}"
			}
			return s
		}
	}
	return string(raw)
}

func decodeOpenAIMessage(in openAIWireMessage) Message {
	msg := Message{Role: in.Role, Content: in.Content}
	for _, tc := range in.ToolCalls {
		typ := tc.Type
		if typ == "" {
			typ = "function"
		}
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:   tc.ID,
			Type: typ,
			Function: ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: parseOpenAIArgs(tc.Function.Arguments),
			},
		})
	}
	return msg
}

func parseOpenAIArgs(raw json.RawMessage) json.RawMessage {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			s = strings.TrimSpace(s)
			if s == "" {
				return json.RawMessage(`{}`)
			}
			return json.RawMessage(s)
		}
	}
	return raw
}
