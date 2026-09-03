package ai

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	toolCallBlock = regexp.MustCompile(`(?is)<tool_call>\s*(.*?)\s*</tool_call>`)
	toolCallTag   = regexp.MustCompile(`(?i)</?tool_call>`)
)

type textToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func normalizeAssistant(msg Message) Message {
	extra := parseTextToolCalls(msg.Content)
	msg.Content = stripToolMarkup(msg.Content)
	if len(extra) == 0 {
		return msg
	}

	seen := make(map[string]bool, len(msg.ToolCalls))
	for _, c := range msg.ToolCalls {
		seen[c.Function.Name+"\n"+string(c.Function.Arguments)] = true
	}
	for _, c := range extra {
		key := c.Function.Name + "\n" + string(c.Function.Arguments)
		if seen[key] {
			continue
		}
		msg.ToolCalls = append(msg.ToolCalls, c)
	}
	return msg
}

func parseTextToolCalls(content string) []ToolCall {
	matches := toolCallBlock.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	out := make([]ToolCall, 0, len(matches))
	for _, m := range matches {
		body := strings.TrimSpace(m[1])
		if body == "" {
			continue
		}
		var raw textToolCall
		if err := json.Unmarshal([]byte(body), &raw); err != nil || raw.Name == "" {
			continue
		}
		out = append(out, ToolCall{
			Type: "function",
			Function: ToolCallFunction{
				Name:      raw.Name,
				Arguments: normalizeArgs(raw.Arguments),
			},
		})
	}
	return out
}

func stripToolMarkup(content string) string {
	s := toolCallBlock.ReplaceAllString(content, "")
	s = toolCallTag.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func normalizeArgs(raw json.RawMessage) json.RawMessage {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	if raw[0] == '"' {
		var inner string
		if err := json.Unmarshal(raw, &inner); err == nil {
			inner = strings.TrimSpace(inner)
			if inner == "" {
				return json.RawMessage(`{}`)
			}
			return json.RawMessage(inner)
		}
	}
	return raw
}
