package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestContextFiles(t *testing.T) {
	names := contextFiles([]Message{{
		Role:    RoleUser,
		Content: "кратко изложи заметку\n\nКонтекст: Serebrovskaya Oblast.md",
	}})
	if len(names) != 1 || names[0] != "Serebrovskaya Oblast.md" {
		t.Fatalf("names = %v", names)
	}

	names = contextFiles([]Message{{
		Role:    RoleUser,
		Content: "прочитай Носки.md и Груша.md",
	}})
	if len(names) != 2 || names[0] != "Носки.md" || names[1] != "Груша.md" {
		t.Fatalf("names = %v", names)
	}
}

func TestAgent_injectsAttachedNote(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Климат: постоянная пасмурность, частые дожди."}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Serebrovskaya Oblast.md", "Климат / атмосфера: Постоянная пасмурность; частые дожди."); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "какой в области климат\n\nКонтекст: Serebrovskaya Oblast.md",
	}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "пасмурность") {
		t.Fatalf("content = %q", result.Content)
	}

	var req struct {
		Messages []Message `json:"messages"`
	}
	if err := json.Unmarshal(seen, &req); err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, m := range req.Messages {
		joined += m.Content
	}
	if !strings.Contains(joined, "Постоянная пасмурность") {
		t.Fatalf("injected messages missing note body: %s", bytes.TrimSpace(seen))
	}
}
