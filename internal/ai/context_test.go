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

func TestAgent_clipsHugeAttachment(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"ок"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Большая.md", strings.Repeat("текст ", 20000)); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	if _, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "перескажи",
	}}, nil, "", "Большая.md"); err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(seen, []byte("текст обрезан")) {
		t.Fatal("clip marker missing")
	}
	if len([]rune(string(seen))) > 30000 {
		t.Fatalf("prompt too big: %d runes", len([]rune(string(seen))))
	}
}

func TestNormalizeAttachments(t *testing.T) {
	names := NormalizeAttachments([]string{" Serebrovskaya Oblast.md ", "Serebrovskaya Oblast.md", "", "Носки.md"})
	if len(names) != 2 || names[0] != "Serebrovskaya Oblast.md" || names[1] != "Носки.md" {
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
		Content: "какой в области климат",
	}}, nil, "", "Serebrovskaya Oblast.md")
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
	for _, m := range req.Messages {
		if m.Role == RoleUser && strings.Contains(m.Content, "Контекст:") {
			t.Fatalf("user message still contains text glue: %q", m.Content)
		}
	}
}

func TestAgent_ignoresFilenameInUserText(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"ок"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Носки.md", "хлопок"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	if _, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "прочитай Носки.md",
	}}, nil, ""); err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(seen, []byte("Текст прикреплённых")) {
		t.Fatalf("filename in user text was treated as attachment: %s", bytes.TrimSpace(seen))
	}
}

func TestAgent_preloadsSearchHits(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Нужно купить носки."}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Носки.md", "Нужно купить носки."); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "носки",
	}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Searched || len(result.Matches) != 1 || result.Matches[0].File != "Носки.md" {
		t.Fatalf("result = %+v", result)
	}
	if !bytes.Contains(seen, []byte("Носки.md")) {
		t.Fatalf("search hits not injected: %s", bytes.TrimSpace(seen))
	}
}

func TestAgent_skipsSearchOnShortReply(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"ок"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Носки.md", "хлопок"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "да",
	}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Searched {
		t.Fatal("short reply should not run search")
	}
	if bytes.Contains(seen, []byte("Поиск по сообщению")) {
		t.Fatalf("search injected on да: %s", bytes.TrimSpace(seen))
	}
}

func TestAgent_answersVaultCountFromDisk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("count query must not call the model")
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("a.md", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := notes.Save("b.md", "two"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "сколько у меня заметок",
	}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "На диске заметок: 2." || !result.System || result.Searched {
		t.Fatalf("result = %+v", result)
	}
}

func TestWriteFact_oneLinePerKind(t *testing.T) {
	got := writeFact([]string{"Баскетбол.md", "Футбол.md", "Теннис.md"}, []string{"Мышки.md", "Мышки.md"})
	want := "Создано: Баскетбол.md, Футбол.md, Теннис.md\nДополнено: Мышки.md"
	if got != want {
		t.Fatalf("fact = %q", got)
	}
}

func TestIsVaultCountQuery(t *testing.T) {
	if !isVaultCountQuery("сколько у меня заметок") || !isVaultCountQuery("Количество заметок") {
		t.Fatal("want count query")
	}
	if isVaultCountQuery("носки") || isVaultCountQuery("сколько стоит") {
		t.Fatal("not a count query")
	}
}

func TestAgent_doesNotReplayEarlierWrites(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 && strings.Contains(string(body), "допиши в заметку про футбол") {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"create_note","arguments":{"title":"Камыш","content":"про камыш"}}},{"type":"function","function":{"name":"append_to_note","arguments":{"filename":"Футбол.md","content":"мяч квадратный"}}}]}}`))
			return
		}
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"create_note","arguments":{"title":"Камыш","content":"про камыш"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"готово"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Футбол.md", "про мяч"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "допиши в заметку про футбол, что мяч квадратный"},
		{Role: RoleSystem, Content: "Дополнено: Футбол.md"},
		{Role: RoleUser, Content: "создай заметку о камыше"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Создано: Камыш.md" || !result.System || len(result.Updated) != 0 {
		t.Fatalf("result = %+v", result)
	}
	list, err := notes.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	football, err := notes.Get("Футбол.md")
	if err != nil || football.Content != "про мяч" {
		t.Fatalf("football = %+v, err = %v", football, err)
	}
}
