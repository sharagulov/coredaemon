package api

import (
	"bufio"
	"bytes"
	"context"
	"errors"
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
}
