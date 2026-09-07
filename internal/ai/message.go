package ai

import (
	"encoding/json"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)

// Message is one chat message for the Ollama API.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
}

// ToolCall is a function call requested by the model.
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and arguments from the model.
type ToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// WithToolSystem prepends the tool-enabled system prompt.
func WithToolSystem(messages []Message, scopeHint string) []Message {
	prompt := ToolSystemPrompt + scopeHint
	out := make([]Message, 0, len(messages)+1)
	out = append(out, Message{Role: RoleSystem, Content: prompt})
	out = append(out, messages...)
	return out
}
