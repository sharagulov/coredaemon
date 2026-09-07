package api

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/ai"
	"github.com/core-daemon/core-daemon/internal/storage"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (flushRecorder) Flush() {}

func TestChat_stream(t *testing.T) {
	step := 0
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"create_note","arguments":{"title":"Hi","content":"body"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"done"}}`))
	}))
	defer ollama.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := ai.NewAgent(ai.New(ollama.URL, "m"), notes)
	mux := http.NewServeMux()
	MountChat(mux, agent)

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
	rec := flushRecorder{httptest.NewRecorder()}
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}

	var events []string
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			events = append(events, strings.TrimPrefix(line, "event: "))
		}
	}
	want := []string{"status", "status", "status", "done"}
	if len(events) != len(want) {
		t.Fatalf("events = %v", events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q", i, events[i], want[i])
		}
	}
}

func TestNormalizeAttachments(t *testing.T) {
	got := ai.NormalizeAttachments([]string{" Serebrovskaya Oblast.md ", "", "Serebrovskaya Oblast.md", "Носки.md"})
	if len(got) != 2 || got[0] != "Serebrovskaya Oblast.md" || got[1] != "Носки.md" {
		t.Fatalf("got %v", got)
	}
}

func TestChat_attachments(t *testing.T) {
	var seen []byte
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"пасмурность"}}`))
	}))
	defer ollama.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Serebrovskaya Oblast.md", "Климат: постоянная пасмурность."); err != nil {
		t.Fatal(err)
	}

	agent := ai.NewAgent(ai.New(ollama.URL, "m"), notes)
	mux := http.NewServeMux()
	MountChat(mux, agent)

	body := bytes.NewBufferString(`{"message":"какой климат","attachments":["Serebrovskaya Oblast.md"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
	rec := flushRecorder{httptest.NewRecorder()}
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(seen, []byte("постоянная пасмурность")) {
		t.Fatalf("ollama request missing attached note: %s", bytes.TrimSpace(seen))
	}
	if bytes.Contains(seen, []byte("Контекст:")) {
		t.Fatalf("text glue still present: %s", bytes.TrimSpace(seen))
	}
}

func TestNormalizeChatMessages(t *testing.T) {
	msgs, err := normalizeChatMessages([]ai.Message{
		{Role: "user", Content: "hi"},
	})
	if err != nil || len(msgs) != 1 {
		t.Fatalf("msgs = %+v, err = %v", msgs, err)
	}

	_, err = normalizeChatMessages([]ai.Message{
		{Role: "assistant", Content: "hi"},
	})
	if err == nil {
		t.Fatal("expected error when last message is not user")
	}

	msgs, err = normalizeChatMessages([]ai.Message{
		{Role: "system", Content: "Создано: a.md"},
		{Role: "user", Content: "допиши туда факт"},
	})
	if err != nil || len(msgs) != 2 || msgs[0].Role != ai.RoleSystem {
		t.Fatalf("msgs = %+v, err = %v", msgs, err)
	}
}

// Ensure agent chat respects request cancellation.
func TestChat_contextCancel(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer ollama.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := ai.NewAgent(ai.New(ollama.URL, "m"), notes)
	mux := http.NewServeMux()
	MountChat(mux, agent)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", body).WithContext(ctx)
	rec := flushRecorder{httptest.NewRecorder()}
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "event: error") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "запрос отменён") {
		t.Fatalf("want cancelled message, body = %s", rec.Body.String())
	}
}

func TestChatErrorMessage(t *testing.T) {
	if got := chatErrorMessage(context.Canceled); got != "запрос отменён" {
		t.Fatalf("canceled = %q", got)
	}
	if got := chatErrorMessage(errors.New("request ollama: connect")); got != "не удалось связаться с Ollama" {
		t.Fatalf("connect = %q", got)
	}
	if got := chatErrorMessage(errors.New("empty response from ollama")); got != "модель вернула пустой ответ" {
		t.Fatalf("empty = %q", got)
	}
	if got := chatErrorMessage(errors.New("tool loop exceeded 8 turns")); got != "агент слишком долго вызывал инструменты" {
		t.Fatalf("loop = %q", got)
	}
	if got := chatErrorMessage(ai.ErrOpenAIUnavailable); got != "OpenAI не настроена: задай OPENAI_API_KEY" {
		t.Fatalf("openai = %q", got)
	}
}

func TestModels_listsProviders(t *testing.T) {
	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := ai.NewAgent(ai.New("http://127.0.0.1:1", "qwen"), notes)
	mux := http.NewServeMux()
	MountModels(mux, agent)

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"id":"ollama"`) || !strings.Contains(body, `"id":"openai"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, `"qwen"`) {
		t.Fatalf("missing ollama model: %s", body)
	}
}

func TestChat_openaiProviderWithoutKey(t *testing.T) {
	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := ai.NewAgent(ai.New("http://127.0.0.1:1", "m"), notes)
	mux := http.NewServeMux()
	MountChat(mux, agent)

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}],"provider":"openai"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
	rec := flushRecorder{httptest.NewRecorder()}
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "event: error") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "OPENAI_API_KEY") {
		t.Fatalf("want key hint, body = %s", rec.Body.String())
	}
}

func TestChat_openaiProviderUsesOpenAI(t *testing.T) {
	ollamaHits := 0
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ollamaHits++
		w.WriteHeader(http.StatusTeapot)
	}))
	defer ollama.Close()

	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"из openai"}}]}`))
	}))
	defer openai.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := ai.NewAgent(ai.New(ollama.URL, "m"), notes)
	agent.UseOpenAI(ai.NewOpenAI(openai.URL, "gpt-4o-mini", "sk-test"), "gpt-4o-mini")
	mux := http.NewServeMux()
	MountChat(mux, agent)

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}],"provider":"openai"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
	rec := flushRecorder{httptest.NewRecorder()}
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if ollamaHits != 0 {
		t.Fatalf("ollama hits = %d", ollamaHits)
	}
	if !strings.Contains(rec.Body.String(), "из openai") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
