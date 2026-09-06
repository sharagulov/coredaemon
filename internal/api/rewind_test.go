package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestRewind_restoreAndTrash(t *testing.T) {
	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("keep.md", "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := notes.Save("gone.md", "new"); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	MountNotes(mux, notes)

	body := bytes.NewBufferString(`{"created":["gone.md"],"previous":{"keep.md":"v1"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/rewind", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || !resp["ok"] {
		t.Fatalf("resp = %s, err = %v", rec.Body.String(), err)
	}

	got, err := notes.Get("keep.md")
	if err != nil || got.Content != "v1" {
		t.Fatalf("keep = %+v, err = %v", got, err)
	}
	if _, err := notes.Get("gone.md"); err != storage.ErrNotFound {
		t.Fatalf("gone should be trashed: %v", err)
	}
}
