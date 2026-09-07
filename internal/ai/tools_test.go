package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestAgent_writeClaimWithoutToolIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Создал заметку Кошки.md"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "создай заметку про кошек"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != NoWriteMsg || !result.System || result.NotesChanged {
		t.Fatalf("result = %+v", result)
	}
	list, err := notes.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
}

func TestAgent_editClaimWithoutToolIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Убрал мяту из заметки Чай.md"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Чай.md", "чай с мятой"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "убери мяту из заметки про чай"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != NoWriteMsg || !result.System || result.NotesChanged {
		t.Fatalf("result = %+v", result)
	}
	note, err := notes.Get("Чай.md")
	if err != nil || note.Content != "чай с мятой" {
		t.Fatalf("note = %+v, err = %v", note, err)
	}
}

func TestAgent_updateNoteRewritesTea(t *testing.T) {
	old := "Чай — это напиток, получаемый путем заваривания листьев чайного растения в горячей воде. Чай широко распространен по всему миру и имеет множество видов, включая зеленый, черный, красный и белый чай. Я недавно в чай добавил мяту. Каждый вид чая имеет свои уникальные характеристики и вкус."
	want := "Чай — это напиток, получаемый путем заваривания листьев чайного растения в горячей воде. Чай широко распространен по всему миру и имеет множество видов, включая зеленый, черный, красный и белый чай. Каждый вид чая имеет свои уникальные характеристики и вкус."

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		args, _ := json.Marshal(map[string]string{"filename": "Чай.md", "content": want})
		fmt.Fprintf(w, `{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"update_note","arguments":%s}}]}}`, args)
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Чай.md", old); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "из этой заметки убери упоминание мяты",
	}}, nil, "", "Чай.md")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Дополнено: Чай.md" || !result.System || !result.NotesChanged {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "Чай.md" || len(result.Created) != 0 {
		t.Fatalf("result = %+v", result)
	}
	note, err := notes.Get("Чай.md")
	if err != nil || note.Content != want || strings.Contains(note.Content, "мяту") {
		t.Fatalf("note = %+v, err = %v", note, err)
	}
}

func TestNoteTools_excludesHoneypots(t *testing.T) {
	names := map[string]bool{}
	for _, tool := range NoteTools() {
		names[tool.Function.Name] = true
	}
	for _, name := range []string{"create_note", "append_to_note", "update_note", "read_note", "search_notes"} {
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
	}, nil, "")
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
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Searched || len(result.Matches) != 1 || result.Matches[0].File != "dog.md" {
		t.Fatalf("result = %+v", result)
	}
	if result.Content != "выдумал десять заметок" {
		t.Fatalf("content = %q", result.Content)
	}
}

func TestAgent_searchStripsMarkdownLinks(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"search_notes","arguments":{"query":"шарика"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Нашёл [Ссылка на dog](dog.md) и [Шарик](dog.md)."}}`))
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
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Content, "](") || strings.Contains(result.Content, "Ссылка") {
		t.Fatalf("content = %q", result.Content)
	}
	if !strings.Contains(result.Content, "Шарик") {
		t.Fatalf("content = %q", result.Content)
	}
}

func TestAgent_linkOnlyReplyFallsBackToFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"- [Ссылка на Шарик](dog.md)"}}`))
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
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Найдено: dog.md" || !result.System {
		t.Fatalf("result = %+v", result)
	}
}

func TestAgent_deleteClaimReplacedByRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Удалено: Мышки.md"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Мышки.md", "полевая, домовая"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "удали заметку"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != storage.BlockedMutationMsg || !result.System || result.NotesChanged {
		t.Fatalf("result = %+v", result)
	}
	if _, err := notes.Get("Мышки.md"); err != nil {
		t.Fatalf("note should remain: %v", err)
	}
}

func TestAgent_nothingFoundDeniedByHits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Ничего не найдено"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Бренд-Tesla.md", "американские машины"); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "что я писал про американские машины"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Найдено: Бренд-Tesla.md" || !result.System {
		t.Fatalf("result = %+v", result)
	}
}

func TestAgent_emptySearchReplacesHallucination(t *testing.T) {
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
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != EmptySearchMsg || !result.System {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Matches) != 0 {
		t.Fatalf("matches = %+v", result.Matches)
	}
}

func TestAgent_readNotHiddenByEmptySearch(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"search_notes","arguments":{"query":"неттакого"}}},{"type":"function","function":{"name":"read_note","arguments":{"filename":"Носки.md"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"в заметке про носки: купить носки"}}`))
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
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "прочитай Носки.md"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "в заметке про носки: купить носки" {
		t.Fatalf("content = %q", result.Content)
	}
}

func TestAgent_repeatSearchReturnsCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"search_notes","arguments":{"query":""}}}]}}`))
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
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "сколько у меня заметок"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "На диске заметок: 2." || !result.System {
		t.Fatalf("result = %+v", result)
	}
}
