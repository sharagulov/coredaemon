package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestNew_trimsTrailingSlash(t *testing.T) {
	c := New("http://localhost:11434/", "m")
	if c.baseURL != "http://localhost:11434" {
		t.Fatalf("baseURL = %q", c.baseURL)
	}
}

func TestClient_ChatOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"hello back"}}`))
	}))
	defer srv.Close()

	client := New(srv.URL, "test-model")
	msg, err := client.ChatOnce(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "hello back" {
		t.Fatalf("content = %q", msg.Content)
	}
}

func TestAgent_toolLoop(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"create_note","arguments":{"title":"Tasks","content":"buy milk"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Создал заметку tasks.md"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "создай заметку"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.NotesChanged {
		t.Fatal("expected notes_changed")
	}
	if result.Content == "" {
		t.Fatal("expected content")
	}

	list, err := notes.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	if len(result.Created) != 1 || result.Created[0] != list[0].Name {
		t.Fatalf("created = %v, file = %q", result.Created, list[0].Name)
	}
}

func TestAgent_textToolCallCreatesNote(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"ronics\n\n{\"name\": \"create_note\", \"arguments\": {\"title\": \"Мерседес\", \"content\": \"рассказ\"}}\n]"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Создал заметку"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "создай заметку про мерседес"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.NotesChanged || len(result.Created) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if strings.Contains(result.Content, "create_note") || strings.Contains(result.Content, "ronics") {
		t.Fatalf("leaked tool text: %q", result.Content)
	}
	list, err := notes.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
}

func TestAgent_blocksTrashNote(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"trash_note","arguments":{"filename":"old.md"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"У меня нет возможности удалять заметки в целях безопасности"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("old.md", "# Old\n"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "убери old.md"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.NotesChanged {
		t.Fatalf("result = %+v", result)
	}
	if _, err := notes.Get("old.md"); err != nil {
		t.Fatalf("note should remain: %v", err)
	}
}
