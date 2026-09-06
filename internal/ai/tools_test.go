package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestNoteTools_excludesHoneypots(t *testing.T) {
	names := map[string]bool{}
	for _, tool := range NoteTools() {
		names[tool.Function.Name] = true
	}
	for _, name := range []string{"create_note", "append_to_note", "read_note", "search_notes"} {
		if !names[name] {
			t.Fatalf("missing tool %q", name)
		}
	}
	for _, name := range []string{"delete_note", "move_note"} {
		if names[name] {
			t.Fatalf("honeypot %q should not be advertised", name)
		}
	}
}

func TestAgent_blocksDeleteNoteHoneypot(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"delete_note","arguments":{"filename":"old.md"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"удаление недоступно"}}`))
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
		{Role: RoleUser, Content: "old.md больше не нужна"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.NotesChanged {
		t.Fatalf("result = %+v", result)
	}
	if result.Content != storage.BlockedMutationMsg {
		t.Fatalf("content = %q", result.Content)
	}
	if _, err := notes.Get("old.md"); err != nil {
		t.Fatalf("note should remain: %v", err)
	}
}

func TestAgent_searchUsesToolHits(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"search_notes","arguments":{"query":"шарика"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"выдумал десять заметок"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("dog.md", "Шарик — собака"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "найди заметки про шарика"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Searched || len(result.Matches) != 1 || result.Matches[0].File != "dog.md" {
		t.Fatalf("result = %+v", result)
	}
	if result.Content != "Найдено: 1\n• dog.md" {
		t.Fatalf("content = %q", result.Content)
	}
}

func TestAgent_emptySearchReplacesModel(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"search_notes","arguments":{"query":"неттакогослова"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"нашёл три секретных файла"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "найди секрет"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != EmptySearchReply || !result.System {
		t.Fatalf("result = %+v", result)
	}
}
