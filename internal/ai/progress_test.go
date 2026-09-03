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
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"thinking", "created", "thinking"}
	if len(phases) != len(want) {
		t.Fatalf("phases = %+v", phases)
	}
	for i, kind := range want {
		if phases[i].Kind != kind {
			t.Fatalf("phase[%d] = %+v, want kind %q", i, phases[i], kind)
		}
	}
	if phases[1].File == "" || phases[1].Title != "T" {
		t.Fatalf("created phase = %+v", phases[1])
	}
}
