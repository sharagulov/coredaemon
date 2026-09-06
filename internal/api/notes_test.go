package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func mountNotes(t *testing.T) (*http.ServeMux, *storage.Notes) {
	t.Helper()
	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	mux := http.NewServeMux()
	MountNotes(mux, notes)
	return mux, notes
}

func serveJSON(t *testing.T, mux http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeAPIError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error: %v, body = %s", err, rec.Body.String())
	}
	return out.Error
}

func TestNotes_listCreateGet(t *testing.T) {
	mux, _ := mountNotes(t)

	rec := serveJSON(t, mux, http.MethodGet, "/api/notes", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list empty status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var empty []storage.NoteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("list = %#v", empty)
	}

	rec = serveJSON(t, mux, http.MethodPost, "/api/notes", `{"name":"test.md","content":"hello"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created storage.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "test.md" || created.Content != "hello" {
		t.Fatalf("created = %+v", created)
	}

	rec = serveJSON(t, mux, http.MethodGet, "/api/notes", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var list []storage.NoteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "test.md" {
		t.Fatalf("list = %+v", list)
	}

	rec = serveJSON(t, mux, http.MethodGet, "/api/notes/test.md", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got storage.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "test.md" || got.Content != "hello" {
		t.Fatalf("got = %+v", got)
	}
}

func TestNotes_putAndPatch(t *testing.T) {
	mux, _ := mountNotes(t)
	if rec := serveJSON(t, mux, http.MethodPost, "/api/notes", `{"name":"keep.md","content":"v1"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seed status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec := serveJSON(t, mux, http.MethodPut, "/api/notes/keep.md", `{"content":"v2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var updated storage.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Content != "v2" {
		t.Fatalf("put note = %+v", updated)
	}

	rec = serveJSON(t, mux, http.MethodPatch, "/api/notes/keep.md", `{"important":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var patched storage.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if !patched.Important || patched.Content != "v2" {
		t.Fatalf("patch note = %+v", patched)
	}
}

func TestNotes_postErrors(t *testing.T) {
	mux, _ := mountNotes(t)

	cases := []struct {
		name string
		body string
		want int
		err  string
	}{
		{name: "invalid json", body: `{not-json`, want: http.StatusBadRequest, err: "invalid json"},
		{name: "empty body", body: "", want: http.StatusBadRequest, err: "request body required"},
		{name: "missing name", body: `{"content":"hello"}`, want: http.StatusBadRequest, err: "name is required"},
		{name: "blank name", body: `{"name":"  ","content":"hello"}`, want: http.StatusBadRequest, err: "name is required"},
		{name: "invalid name", body: `{"name":"../x.md","content":"hello"}`, want: http.StatusBadRequest, err: "invalid note name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveJSON(t, mux, http.MethodPost, "/api/notes", tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
			if got := decodeAPIError(t, rec); got != tc.err {
				t.Fatalf("error = %q, want %q", got, tc.err)
			}
		})
	}
}

func TestNotes_postExceedsMaxBytes(t *testing.T) {
	mux, _ := mountNotes(t)
	body := `{"name":"big.md","content":"` + strings.Repeat("a", maxBody) + `"}`

	rec := serveJSON(t, mux, http.MethodPost, "/api/notes", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := decodeAPIError(t, rec); got != "invalid json" {
		t.Fatalf("error = %q", got)
	}
}

func TestNotes_putAndPatchErrors(t *testing.T) {
	mux, _ := mountNotes(t)
	if rec := serveJSON(t, mux, http.MethodPost, "/api/notes", `{"name":"keep.md","content":"v1"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seed status = %d, body = %s", rec.Code, rec.Body.String())
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
		err    string
	}{
		{name: "put invalid json", method: http.MethodPut, path: "/api/notes/keep.md", body: `{`, want: http.StatusBadRequest, err: "invalid json"},
		{name: "put empty body", method: http.MethodPut, path: "/api/notes/keep.md", body: "", want: http.StatusBadRequest, err: "request body required"},
		{name: "patch invalid json", method: http.MethodPatch, path: "/api/notes/keep.md", body: `{`, want: http.StatusBadRequest, err: "invalid json"},
		{name: "patch empty object", method: http.MethodPatch, path: "/api/notes/keep.md", body: `{}`, want: http.StatusBadRequest, err: "nothing to update"},
		{name: "patch missing", method: http.MethodPatch, path: "/api/notes/missing.md", body: `{"important":true}`, want: http.StatusNotFound, err: "note not found"},
		{name: "get missing", method: http.MethodGet, path: "/api/notes/missing.md", body: "", want: http.StatusNotFound, err: "note not found"},
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
