package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func TestSearch_query(t *testing.T) {
	mux, notes := mountNotes(t)
	if _, err := notes.Save("hit.md", "hello test world"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		path     string
		want     int
		wantLen  int
		wantFile string
	}{
		{name: "match", path: "/api/search?q=test", want: http.StatusOK, wantLen: 1, wantFile: "hit.md"},
		{name: "empty q", path: "/api/search?q=", want: http.StatusOK, wantLen: 0},
		{name: "missing q", path: "/api/search", want: http.StatusOK, wantLen: 0},
		{name: "no hits", path: "/api/search?q=неттакогослова", want: http.StatusOK, wantLen: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveJSON(t, mux, http.MethodGet, tc.path, "")
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
			var hits []storage.SearchHit
			if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
				t.Fatalf("decode: %v, body = %s", err, rec.Body.String())
			}
			if hits == nil {
				t.Fatal("hits must be [] not null")
			}
			if len(hits) != tc.wantLen {
				t.Fatalf("hits = %+v", hits)
			}
			if tc.wantFile != "" && hits[0].File != tc.wantFile {
				t.Fatalf("file = %q, want %q", hits[0].File, tc.wantFile)
			}
		})
	}
}
