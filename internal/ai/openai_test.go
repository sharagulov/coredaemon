package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestParseProvider(t *testing.T) {
	got, err := ParseProvider("")
	if err != nil || got != ProviderOllama {
		t.Fatalf("empty = %q, err = %v", got, err)
	}
	got, err = ParseProvider("OpenAI")
	if err != nil || got != ProviderOpenAI {
		t.Fatalf("openai = %q, err = %v", got, err)
	}
	if _, err := ParseProvider("claude"); err == nil {
		t.Fatal("expected unknown provider")
	}
}

func TestOpenAIClient_ChatOnce_toolRoundtrip(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"create_note","arguments":"{\"title\":\"Чай\",\"content\":\"напиток\"}"}}]}}]}`))
	}))
	defer srv.Close()

	msg, err := NewOpenAI(srv.URL, "gpt-4o-mini", "sk-test").ChatOnce(context.Background(), []Message{
		{Role: RoleUser, Content: "создай заметку про чай"},
	}, NoteTools())
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "create_note" || msg.ToolCalls[0].ID != "call_1" {
		t.Fatalf("msg = %+v", msg)
	}
	var args struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(msg.ToolCalls[0].Function.Arguments, &args); err != nil || args.Title != "Чай" {
		t.Fatalf("args = %+v, err = %v, raw = %s", args, err, msg.ToolCalls[0].Function.Arguments)
	}

	var req openAIRequest
	if err := json.Unmarshal(seen, &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "gpt-4o-mini" || req.Temperature != 0 || len(req.Tools) == 0 {
		t.Fatalf("req = %+v", req)
	}
	if strings.Contains(string(seen), "num_ctx") {
		t.Fatalf("ollama options leaked: %s", seen)
	}
}

func TestOpenAIClient_encodesToolResults(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"готово"}}]}`))
	}))
	defer srv.Close()

	_, err := NewOpenAI(srv.URL, "gpt-4o-mini", "sk-test").ChatOnce(context.Background(), []Message{
		{Role: RoleAssistant, ToolCalls: []ToolCall{{
			ID:   "call_9",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "create_note",
				Arguments: json.RawMessage(`{"title":"Чай","content":"x"}`),
			},
		}}},
		{Role: RoleTool, ToolName: "create_note", Content: `{"status":"success"}`},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(seen), `"tool_call_id":"call_9"`) {
		t.Fatalf("missing tool_call_id: %s", seen)
	}
	var req openAIRequest
	if err := json.Unmarshal(seen, &req); err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) < 1 || len(req.Messages[0].ToolCalls) != 1 {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if !strings.HasPrefix(req.Messages[0].ToolCalls[0].Function.Arguments, "{") {
		t.Fatalf("arguments = %q", req.Messages[0].ToolCalls[0].Function.Arguments)
	}
}

func TestAgent_ChatUsing_openaiCreatesNote(t *testing.T) {
	step := 0
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"create_note","arguments":"{\"title\":\"Чай\",\"content\":\"напиток\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"готово"}}]}`))
	}))
	defer srvClose(t, openai)

	ollamaHits := 0
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ollamaHits++
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srvClose(t, ollama)

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New(ollama.URL, "m"), notes)
	agent.UseOpenAI(NewOpenAI(openai.URL, "gpt-4o-mini", "sk-test"), "gpt-4o-mini")
	result, err := agent.ChatUsing(context.Background(), ProviderOpenAI, []Message{
		{Role: RoleUser, Content: "создай заметку про чай"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if ollamaHits != 0 {
		t.Fatalf("ollama was called %d times", ollamaHits)
	}
	if !result.NotesChanged || len(result.Created) != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestAgent_Chat_staysOnOllamaWhenOpenAIConfigured(t *testing.T) {
	ollamaHits := 0
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ollamaHits++
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"локально"}}`))
	}))
	defer srvClose(t, ollama)

	openaiHits := 0
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openaiHits++
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srvClose(t, openai)

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New(ollama.URL, "m"), notes)
	agent.UseOpenAI(NewOpenAI(openai.URL, "gpt-4o-mini", "sk-test"), "gpt-4o-mini")
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "привет"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "локально" || openaiHits != 0 || ollamaHits != 1 {
		t.Fatalf("result = %+v ollama=%d openai=%d", result, ollamaHits, openaiHits)
	}
}

func TestAgent_ChatUsing_openaiUnavailable(t *testing.T) {
	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New("http://127.0.0.1:1", "m"), notes)
	_, err = agent.ChatUsing(context.Background(), ProviderOpenAI, []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil, "")
	if err != ErrOpenAIUnavailable {
		t.Fatalf("err = %v", err)
	}
	list := agent.Providers()
	if len(list) != 2 || list[0].ID != ProviderOllama || !list[0].Available || list[1].Available {
		t.Fatalf("providers = %+v", list)
	}
}

func srvClose(t *testing.T, srv *httptest.Server) {
	t.Helper()
	srv.Close()
}
