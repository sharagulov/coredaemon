package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestSections_createListDelete(t *testing.T) {
	mux, _ := mountNotes(t)

	rec := serveJSON(t, mux, http.MethodGet, "/api/sections", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list empty status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var empty []storage.Section
	if err := json.Unmarshal(rec.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("sections = %#v", empty)
	}

	rec = serveJSON(t, mux, http.MethodPost, "/api/sections", `{"name":"Работа"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created storage.Section
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Name != "Работа" {
		t.Fatalf("created = %+v", created)
	}

	rec = serveJSON(t, mux, http.MethodGet, "/api/sections", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var list []storage.Section
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v", list)
	}

	rec = serveJSON(t, mux, http.MethodDelete, "/api/sections/"+created.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = serveJSON(t, mux, http.MethodGet, "/api/sections", "")
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("after delete = %+v", list)
	}
}

func TestSections_errors(t *testing.T) {
	mux, _ := mountNotes(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
		err    string
	}{
		{name: "invalid json", method: http.MethodPost, path: "/api/sections", body: `{`, want: http.StatusBadRequest, err: "invalid json"},
		{name: "empty body", method: http.MethodPost, path: "/api/sections", body: "", want: http.StatusBadRequest, err: "request body required"},
		{name: "empty name", method: http.MethodPost, path: "/api/sections", body: `{"name":""}`, want: http.StatusBadRequest, err: "invalid section name"},
		{name: "blank name", method: http.MethodPost, path: "/api/sections", body: `{"name":"   "}`, want: http.StatusBadRequest, err: "invalid section name"},
		{name: "reserved trash", method: http.MethodPost, path: "/api/sections", body: `{"name":"trash"}`, want: http.StatusBadRequest, err: "invalid section name"},
		{name: "reserved all", method: http.MethodPost, path: "/api/sections", body: `{"name":"all"}`, want: http.StatusBadRequest, err: "invalid section name"},
		{name: "slash", method: http.MethodPost, path: "/api/sections", body: `{"name":"foo/bar"}`, want: http.StatusBadRequest, err: "invalid section name"},
		{name: "backslash", method: http.MethodPost, path: "/api/sections", body: `{"name":"foo\\bar"}`, want: http.StatusBadRequest, err: "invalid section name"},
		{name: "dotdot", method: http.MethodPost, path: "/api/sections", body: `{"name":"../secret"}`, want: http.StatusBadRequest, err: "invalid section name"},
		{name: "delete missing", method: http.MethodDelete, path: "/api/sections/missing", body: "", want: http.StatusNotFound, err: "section not found"},
		{name: "delete reserved", method: http.MethodDelete, path: "/api/sections/trash", body: "", want: http.StatusBadRequest, err: "invalid section"},
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

func TestSections_rejectsPathNames(t *testing.T) {
	dir := t.TempDir()
	notes, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	mux := http.NewServeMux()
	MountNotes(mux, notes)

	for _, body := range []string{`{"name":"foo/bar"}`, `{"name":"../secret"}`} {
		rec := serveJSON(t, mux, http.MethodPost, "/api/sections", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, resp = %s", body, rec.Code, rec.Body.String())
		}
		if got := decodeAPIError(t, rec); got != "invalid section name" {
			t.Fatalf("error = %q", got)
		}
	}

	list, err := notes.ListSections()
	if err != nil || len(list) != 0 {
		t.Fatalf("sections = %+v, err = %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".sections.json")); !os.IsNotExist(err) {
		t.Fatalf("sections file should not be written: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Fatalf("unexpected directory %q", e.Name())
		}
	}
}

func TestSections_conflict(t *testing.T) {
	mux, _ := mountNotes(t)
	if rec := serveJSON(t, mux, http.MethodPost, "/api/sections", `{"name":"Ideas"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec := serveJSON(t, mux, http.MethodPost, "/api/sections", `{"name":"Ideas"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := decodeAPIError(t, rec); got != "section already exists" {
		t.Fatalf("error = %q", got)
	}
}
