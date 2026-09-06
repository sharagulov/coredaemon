package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestAgent_emitsProgress(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"create_note","arguments":{"title":"T","content":"x"}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"ok"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	var phases []Phase
	agent := NewAgent(New(srv.URL, "m"), notes)
	_, err = agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "создай заметку"},
	}, func(p Phase) {
		phases = append(phases, p)
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(phases) == 0 || phases[0].Kind != "thinking" {
		t.Fatalf("phases = %+v", phases)
	}
	var created []Phase
	for _, p := range phases {
		if p.Kind == "created" {
			created = append(created, p)
		}
	}
	if len(created) != 1 {
		t.Fatalf("created phases = %+v", created)
	}
	if created[0].File == "" || created[0].Title != "T" {
		t.Fatalf("created phase = %+v", created[0])
	}
}
