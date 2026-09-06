package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestChats_putAndGet(t *testing.T) {
	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	mux := http.NewServeMux()
	MountChats(mux, notes)

	body := bytes.NewBufferString(`{"activeId":"c1","chats":[{"id":"c1","title":"привет","messages":[{"role":"user","content":"hi"}]}]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/chats", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}

	var store storage.ChatStore
	if err := json.Unmarshal(rec.Body.Bytes(), &store); err != nil {
		t.Fatal(err)
	}
	if store.ActiveID != "c1" || len(store.Chats) != 1 || store.Chats[0].Title != "привет" {
		t.Fatalf("store = %+v", store)
	}
}
