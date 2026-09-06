package ai

import "testing"

func TestParseTextToolCalls_bareJSON(t *testing.T) {
	content := "ronics\n\n{\"name\": \"create_note\", \"arguments\": {\"title\": \"Мерседес, БМВ и Ауда\", \"content\": \"рассказ\"}}\n\n]"
	calls := parseTextToolCalls(content)
	if len(calls) != 1 || calls[0].Function.Name != "create_note" {
		t.Fatalf("calls = %+v", calls)
	}
	if got := stripToolMarkup(content); got != "" {
		t.Fatalf("strip = %q", got)
	}
}

func TestParseTextToolCalls_array(t *testing.T) {
	content := "[{\"name\":\"search_notes\",\"arguments\":{\"query\":\"\"}}]"
	calls := parseTextToolCalls(content)
	if len(calls) != 1 || calls[0].Function.Name != "search_notes" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestParseTextToolCalls_unclosed(t *testing.T) {
	content := "<tool_call>\n{\"name\":\"read_note\",\"arguments\":{\"filename\":\"a.md\"}}\n"
	calls := parseTextToolCalls(content)
	if len(calls) != 1 || calls[0].Function.Name != "read_note" {
		t.Fatalf("calls = %+v", calls)
	}
	if got := stripToolMarkup(content); got != "" {
		t.Fatalf("strip = %q", got)
	}
}

func TestParseTextToolCalls_hermes(t *testing.T) {
	content := "<tool_call>\ncreate_note\n{\"title\":\"T\",\"content\":\"x\"}\n</tool_call>"
	calls := parseTextToolCalls(content)
	if len(calls) != 1 || calls[0].Function.Name != "create_note" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestParseTextToolCalls(t *testing.T) {
	content := "Сейчас поищу.\n<tool_call>\n{\"name\": \"search_notes\", \"arguments\": {\"query\": \"промышленность\"}}\n</tool_call>"
	calls := parseTextToolCalls(content)
	if len(calls) != 1 || calls[0].Function.Name != "search_notes" {
		t.Fatalf("calls = %+v", calls)
	}
	if string(calls[0].Function.Arguments) != `{"query": "промышленность"}` {
		t.Fatalf("args = %s", calls[0].Function.Arguments)
	}
	if got := stripToolMarkup(content); got != "Сейчас поищу." {
		t.Fatalf("strip = %q", got)
	}
}

func TestNormalizeAssistant_mergesTextCalls(t *testing.T) {
	msg := normalizeAssistant(Message{
		Role:    RoleAssistant,
		Content: `<tool_call>{"name":"read_note","arguments":{"filename":"a.md"}}</tool_call>`,
	})
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "read_note" {
		t.Fatalf("msg = %+v", msg)
	}
	if msg.Content != "" {
		t.Fatalf("content = %q", msg.Content)
	}
}

func TestGroundedContent(t *testing.T) {
	got := groundedContent("выдумал три заметки", true, nil, false)
	if got != "По этому запросу в заметках ничего не найдено." {
		t.Fatalf("got %q", got)
	}
	if groundedContent("ok", false, nil, false) != "ok" {
		t.Fatal("passthrough")
	}
}
