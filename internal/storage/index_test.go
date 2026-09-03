package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTest(t *testing.T) *Notes {
	t.Helper()
	n, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return n
}

func TestSearch_findsSavedNote(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("pie.md", "Рецепт пирога: мука, яблоки, сахар"); err != nil {
		t.Fatal(err)
	}

	hits, err := n.Search("пирога")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].File != "pie.md" {
		t.Fatalf("hits = %+v", hits)
	}
	if !strings.Contains(hits[0].Snippet, "пирога") {
		t.Fatalf("snippet = %q", hits[0].Snippet)
	}
}

func TestSearch_rebuildsOnOpen(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "automotive-brand", "REUS")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte("встреча в четверг"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "Identity.md"), []byte("бренд REUS"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })

	hits, err := n.Search("четверг")
	if err != nil || len(hits) != 1 || hits[0].File != "plan.md" {
		t.Fatalf("hits = %+v, err = %v", hits, err)
	}

	nestedHits, err := n.Search("REUS")
	if err != nil || len(nestedHits) != 1 || nestedHits[0].File != "automotive-brand/REUS/Identity.md" {
		t.Fatalf("nested hits = %+v, err = %v", nestedHits, err)
	}
}

func TestSearch_emptyAndSafeQuery(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("a.md", "hello"); err != nil {
		t.Fatal(err)
	}

	hits, err := n.Search("   ")
	if err != nil || len(hits) != 0 {
		t.Fatalf("empty query: %+v, err = %v", hits, err)
	}

	hits, err = n.Search(`^^^ """`)
	if err != nil || len(hits) != 0 {
		t.Fatalf("noise query: %+v, err = %v", hits, err)
	}
}

func TestSearch_tool(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("tea.md", "заварить чай"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("search_notes", []byte(`{"query":"чай"}`))
	if err != nil || res.Status != "success" || len(res.Hits) != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if res.Hits[0].File != "tea.md" || res.Found != 1 {
		t.Fatalf("file = %q found = %d", res.Hits[0].File, res.Found)
	}

	empty, err := n.RunTool("search_notes", []byte(`{"query":"промышленность"}`))
	if err != nil || empty.Status != "success" || empty.Found != 0 || len(empty.Hits) != 0 {
		t.Fatalf("empty search = %+v, err = %v", empty, err)
	}
}

func TestFtsQuery(t *testing.T) {
	if got := ftsQuery("рецепт пирога?"); got != "рецепт пирога" {
		t.Fatalf("got %q", got)
	}
	if ftsQuery("***") != "" {
		t.Fatal("expected empty")
	}
}
