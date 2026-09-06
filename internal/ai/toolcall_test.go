package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

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

func TestEncodeToolResult_searchEmpty(t *testing.T) {
	body := encodeToolResult("search_notes", storage.ToolResult{Status: "success", Query: "tasks"}, 22)
	if !strings.Contains(string(body), `"hits":[]`) || !strings.Contains(string(body), `"found":0`) || !strings.Contains(string(body), `"vault":22`) {
		t.Fatalf("body = %s", body)
	}
}

func TestCleanReply_stripsFileLinks(t *testing.T) {
	got := cleanReply("Созданы заметки:\n[Ссылка на Мышка 1](Мышка-1.md)\n[[Мышка 2]]\n[полевая](Мышь-полевая.md)")
	if strings.Contains(got, "](") || strings.Contains(got, "[[") {
		t.Fatalf("links left: %q", got)
	}
	if !strings.Contains(got, "полевая") || !strings.Contains(got, "Мышка 2") {
		t.Fatalf("useful labels dropped: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "ссылка") {
		t.Fatalf("link label kept: %q", got)
	}
}

func TestCleanReply_dropsBulletsLeftByLinks(t *testing.T) {
	got := cleanReply("Готово:\n- [Ссылка на Мышь полевая](Мышь-полевая.md)\n- ![Ссылка](Мышь-домовая.md)\n1. [Ссылка](Мышь-летучая.md)")
	if got != "Готово:" {
		t.Fatalf("clean = %q", got)
	}
	code := "Пример:\n```\nx = 1\n```"
	if cleanReply(code) != code {
		t.Fatalf("code fence broken: %q", cleanReply(code))
	}
}

func TestClaimsMutation(t *testing.T) {
	for _, s := range []string{"Удалено: Мышки.md", "Заметка удалена", "Я перенёс заметку в архив"} {
		if !claimsMutation(s) {
			t.Fatalf("missed claim: %q", s)
		}
	}
	for _, s := range []string{
		"Ты перенёс вещи в гараж",
		"Удаление заметок недоступно",
		"Удалить заметку можно вручную",
		"Создано: a.md",
	} {
		if claimsMutation(s) {
			t.Fatalf("false claim: %q", s)
		}
	}
}

func TestToolKey_normalizesEmptySearch(t *testing.T) {
	a := toolKey("search_notes", json.RawMessage(`{}`))
	b := toolKey("search_notes", json.RawMessage(`{"query":"*"}`))
	c := toolKey("search_notes", json.RawMessage(`{"query":""}`))
	if a != b || b != c {
		t.Fatalf("keys = %q %q %q", a, b, c)
	}
}
