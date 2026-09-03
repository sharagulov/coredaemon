package ai

import "testing"

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
