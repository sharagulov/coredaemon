package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestTrash_moveAndRestore(t *testing.T) {
	mux, notes := mountNotes(t)
	if _, err := notes.Save("gone.md", "body"); err != nil {
		t.Fatal(err)
	}

	rec := serveJSON(t, mux, http.MethodGet, "/api/trash", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list empty status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var empty []storage.TrashItem
	if err := json.Unmarshal(rec.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("trash = %#v", empty)
	}

	rec = serveJSON(t, mux, http.MethodPost, "/api/trash", `{"name":"gone.md"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("trash status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var moved struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &moved); err != nil || moved.Name != "gone.md" {
		t.Fatalf("trash resp = %s, err = %v", rec.Body.String(), err)
	}
	if _, err := notes.Get("gone.md"); err != storage.ErrNotFound {
		t.Fatalf("note should be gone: %v", err)
	}

	rec = serveJSON(t, mux, http.MethodGet, "/api/trash", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var items []storage.TrashItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "gone.md" || items[0].ID == "" {
		t.Fatalf("trash list = %+v", items)
	}

	rec = serveJSON(t, mux, http.MethodPost, "/api/trash/"+items[0].ID+"/restore", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var restored storage.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Name != "gone.md" || restored.Content != "body" {
		t.Fatalf("restored = %+v", restored)
	}
	if _, err := notes.Get("gone.md"); err != nil {
		t.Fatalf("note should be back: %v", err)
	}
}

func TestTrash_errors(t *testing.T) {
	mux, _ := mountNotes(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
		err    string
	}{
		{name: "trash missing note", method: http.MethodPost, path: "/api/trash", body: `{"name":"missing.md"}`, want: http.StatusNotFound, err: "note not found"},
		{name: "trash invalid json", method: http.MethodPost, path: "/api/trash", body: `{`, want: http.StatusBadRequest, err: "invalid json"},
		{name: "trash empty body", method: http.MethodPost, path: "/api/trash", body: "", want: http.StatusBadRequest, err: "request body required"},
		{name: "trash invalid name", method: http.MethodPost, path: "/api/trash", body: `{"name":"../x.md"}`, want: http.StatusBadRequest, err: "invalid note name"},
		{name: "restore missing id", method: http.MethodPost, path: "/api/trash/9999999999999/restore", body: "", want: http.StatusNotFound, err: "note not found"},
		{name: "restore invalid id", method: http.MethodPost, path: "/api/trash/not-an-id/restore", body: "", want: http.StatusBadRequest, err: "invalid trash id"},
		{name: "get missing trash", method: http.MethodGet, path: "/api/trash/9999999999999", body: "", want: http.StatusNotFound, err: "note not found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveJSON(t, mux, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
			if got := decodeAPIError(t, rec); got != tc.err {
				t.Fatalf("error = %q, want %q", got, tc.err)
			}
		})
	}
}
