package ai

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

var (
	toolCallBlock = regexp.MustCompile(`(?is)<tool_call>\s*(.*?)\s*</tool_call>`)
	toolCallTag   = regexp.MustCompile(`(?i)</?tool_call>`)
	toolCallOpen  = regexp.MustCompile(`(?i)<tool_call>`)
	markdownLink  = regexp.MustCompile(`!?\[([^\[\]]*)\]\([^)]*\)`)
	wikiLink      = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)
	linkOnlyLine  = regexp.MustCompile(`(?m)^[ \t]*Ссылка на[^\n]*\n?`)
	// letterlessLine matches a line without letters, e.g. the "- " left by a stripped link.
	// Backticks are excluded so code fences survive.
	letterlessLine = regexp.MustCompile("(?m)^[^\\p{L}\n`]+$\n?")
	// doneMutation matches a finished delete or move, e.g. the "Удалено: Мышки.md" the model
	// copies from earlier "Создано: …" reports. Nouns stay out of it, so an honest
	// "удаление недоступно" reaches the user as the model wrote it.
	doneMutation = regexp.MustCompile(`(?i)удалил|переместил|перен[её]с|(?:удал|перемещ|перенес)[ёе]н[аоы]?(?:$|[^\p{L}])`)
	// doneWrite matches a finished write ("Создал заметку Кошки.md"). Nouns and infinitives stay
	// out of it, so "создание заметок" and "что добавить" are not treated as reports.
	doneWrite  = regexp.MustCompile(`(?i)создал|записал|дописал|добавил|убрал|исправил|заменил|переписал|(?:создан|записан|дополнен|изменен|исправлен|заменен)[аоы]?(?:$|[^\p{L}])`)
	extraBlank = regexp.MustCompile(`\n{3,}`)
)

var textToolNames = map[string]bool{
	"create_note":    true,
	"append_to_note": true,
	"update_note":    true,
	"read_note":      true,
	"search_notes":   true,
	"trash_note":     true,
	"delete_note":    true,
	"remove_note":    true,
	"move_note":      true,
}

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
		seen[key] = true
		msg.ToolCalls = append(msg.ToolCalls, c)
	}
	return msg
}

func parseTextToolCalls(content string) []ToolCall {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	var out []ToolCall
	for _, m := range toolCallBlock.FindAllStringSubmatch(content, -1) {
		out = append(out, parseToolCallBody(m[1])...)
	}
	if !toolCallBlock.MatchString(content) {
		if loc := toolCallOpen.FindStringIndex(content); loc != nil {
			body := toolCallTag.ReplaceAllString(content[loc[1]:], "")
			out = append(out, parseToolCallBody(body)...)
		}
	}
	out = append(out, extractJSONToolCalls(content)...)
	return dedupeCalls(out)
}

func dedupeCalls(calls []ToolCall) []ToolCall {
	if len(calls) < 2 {
		return calls
	}
	seen := make(map[string]bool, len(calls))
	out := make([]ToolCall, 0, len(calls))
	for _, c := range calls {
		key := c.Function.Name + "\n" + string(c.Function.Arguments)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

func parseToolCallBody(body string) []ToolCall {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	if calls := parseToolJSON([]byte(body)); len(calls) > 0 {
		return calls
	}

	if i := strings.Index(body, "("); i > 0 && strings.HasSuffix(body, ")") {
		name := strings.TrimSpace(body[:i])
		if textToolNames[name] {
			args := strings.TrimSpace(body[i+1 : len(body)-1])
			return []ToolCall{makeTextCall(name, json.RawMessage(args))}
		}
	}

	name, rest, ok := strings.Cut(body, "\n")
	name = strings.TrimSpace(name)
	rest = strings.TrimSpace(rest)
	if ok && textToolNames[name] && strings.HasPrefix(rest, "{") {
		return []ToolCall{makeTextCall(name, json.RawMessage(rest))}
	}
	return nil
}

func extractJSONToolCalls(s string) []ToolCall {
	var out []ToolCall
	for i := 0; i < len(s); i++ {
		if s[i] != '{' && s[i] != '[' {
			continue
		}
		raw, n := decodeJSONAt(s[i:])
		if n == 0 {
			continue
		}
		calls := parseToolJSON(raw)
		if len(calls) > 0 {
			out = append(out, calls...)
			i += n - 1
			continue
		}
	}
	return out
}

func decodeJSONAt(s string) (json.RawMessage, int) {
	dec := json.NewDecoder(strings.NewReader(s))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, 0
	}
	return raw, int(dec.InputOffset())
}

func parseToolJSON(raw json.RawMessage) []ToolCall {
	raw = json.RawMessage(bytes.TrimSpace(raw))
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		var items []textToolCall
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil
		}
		out := make([]ToolCall, 0, len(items))
		for _, item := range items {
			if c, ok := textCall(item); ok {
				out = append(out, c)
			}
		}
		return out
	}
	var item textToolCall
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil
	}
	if c, ok := textCall(item); ok {
		return []ToolCall{c}
	}
	return nil
}

func textCall(item textToolCall) (ToolCall, bool) {
	if !textToolNames[item.Name] {
		return ToolCall{}, false
	}
	return makeTextCall(item.Name, item.Arguments), true
}

func makeTextCall(name string, args json.RawMessage) ToolCall {
	return ToolCall{
		Type: "function",
		Function: ToolCallFunction{
			Name:      name,
			Arguments: normalizeArgs(args),
		},
	}
}

func cleanReply(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = markdownLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := markdownLink.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		label := strings.TrimSpace(sub[1])
		if label == "" {
			return ""
		}
		if strings.HasPrefix(strings.ToLower(label), "ссылка") {
			return ""
		}
		return label
	})
	s = wikiLink.ReplaceAllString(s, "$1")
	s = linkOnlyLine.ReplaceAllString(s, "")
	s = letterlessLine.ReplaceAllString(s, "")
	s = extraBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// claimsMutation reports whether the reply announces a delete or move of a note. Deletion
// never runs through the model, so such a reply is false whenever the vault stayed untouched.
// The note mention keeps retold note bodies ("перенёс вещи в гараж") out of the check.
func claimsMutation(s string) bool {
	if !doneMutation.MatchString(s) {
		return false
	}
	low := strings.ToLower(s)
	return strings.Contains(low, ".md") || strings.Contains(low, "заметк")
}

// claimsWrite reports whether the reply announces a note write. The note mention keeps retold
// note bodies ("ты добавил соль в тесто") out of the check.
func claimsWrite(s string) bool {
	if !doneWrite.MatchString(s) {
		return false
	}
	low := strings.ToLower(s)
	return strings.Contains(low, ".md") || strings.Contains(low, "заметк")
}

// claimsNothingFound reports whether the reply denies hits the tools actually returned.
func claimsNothingFound(s string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(EmptySearchMsg))
}

func stripToolMarkup(content string) string {
	s := toolCallBlock.ReplaceAllString(content, "")
	s = toolCallTag.ReplaceAllString(s, "")
	s = stripJSONToolBlobs(s)
	return strings.TrimSpace(s)
}

func stripJSONToolBlobs(s string) string {
	var b strings.Builder
	removed := false
	for i := 0; i < len(s); {
		if s[i] == '{' || s[i] == '[' {
			raw, n := decodeJSONAt(s[i:])
			if n > 0 && len(parseToolJSON(raw)) > 0 {
				removed = true
				i += n
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	out := strings.TrimSpace(b.String())
	if !removed {
		return out
	}
	return dropToolNoise(out)
}

func dropToolNoise(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]{}, \n\t\r")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.ContainsAny(s, ".!?") || strings.ContainsAny(s, " \n\t") {
		return s
	}
	if len([]rune(s)) <= 12 {
		return ""
	}
	return s
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
