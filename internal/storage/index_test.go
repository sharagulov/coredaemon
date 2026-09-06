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

	blank, err := n.RunTool("search_notes", []byte(`{}`))
	if err != nil || blank.Status != "success" || blank.Found != 0 || len(blank.Hits) != 0 {
		t.Fatalf("empty query = %+v, err = %v", blank, err)
	}

	byFile, err := n.RunTool("search_notes", []byte(`{"query":"tea.md"}`))
	if err != nil || byFile.Status != "success" || byFile.Found != 1 || byFile.Hits[0].File != "tea.md" {
		t.Fatalf("filename query = %+v, err = %v", byFile, err)
	}

	commandOnly, err := n.RunTool("search_notes", []byte(`{"query":"найди заметки"}`))
	if err != nil || commandOnly.Status != "success" || commandOnly.Found != 0 || len(commandOnly.Hits) != 0 {
		t.Fatalf("command-only query = %+v, err = %v", commandOnly, err)
	}
}

func TestSearch_tool_lookalikeFilename(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("Носки.md", "купить носки"); err != nil {
		t.Fatal(err)
	}
	// Latin N + Cyrillic оски — так иногда приходит query от модели.
	res, err := n.RunTool("search_notes", []byte(`{"query":"Nоски.md"}`))
	if err != nil || res.Found != 1 || res.Hits[0].File != "Носки.md" {
		t.Fatalf("lookalike search = %+v, err = %v", res, err)
	}
	read, err := n.RunTool("read_note", []byte(`{"filename":"Nоски.md"}`))
	if err != nil || read.Status != "success" || read.File != "Носки.md" || !strings.Contains(read.Content, "носки") {
		t.Fatalf("lookalike read = %+v, err = %v", read, err)
	}
}

func TestFtsQuery(t *testing.T) {
	if got := ftsQuery("рецепт пирога?"); got != "рецепт* пирог*" {
		t.Fatalf("got %q", got)
	}
	if got := ftsQuery("найди заметки про шарика"); got != "шарик*" {
		t.Fatalf("got %q", got)
	}
	if got := ftsQuery("небоскрёбы"); got != "небоскреб*" {
		t.Fatalf("ё fold = %q", got)
	}
	if ftsQuery("небоскребы") != "небоскреб*" {
		t.Fatal("е and ё must stem the same")
	}
	if ftsQuery("***") != "" {
		t.Fatal("expected empty")
	}
	if got := ftsQuery("Nоски.md"); got != "носк*" {
		t.Fatalf("lookalike query = %q", got)
	}
	if got := ftsQuery("tea.md"); got != "tea*" {
		t.Fatalf("filename query = %q", got)
	}
	if got := ftsQuery("test"); got != "test*" {
		t.Fatalf("latin query = %q", got)
	}
	if !Searchable("кошки") || !Searchable("носки") || Searchable("да") {
		t.Fatal("Searchable")
	}
}

func TestSearch_inflectedAndCommandWords(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("dog.md", "Шарик — собака"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("tz.md", "Техническое задание на модуль"); err != nil {
		t.Fatal(err)
	}

	hits, err := n.Search("шарика")
	if err != nil || len(hits) != 1 || hits[0].File != "dog.md" {
		t.Fatalf("шарика = %+v, err = %v", hits, err)
	}
	hits, err = n.Search("найди заметки про шарика")
	if err != nil || len(hits) != 1 || hits[0].File != "dog.md" {
		t.Fatalf("command query = %+v, err = %v", hits, err)
	}
	hits, err = n.Search("техническим заданием")
	if err != nil || len(hits) != 1 || hits[0].File != "tz.md" {
		t.Fatalf("tz = %+v, err = %v", hits, err)
	}
}

func TestSearch_titleWordInPunctuatedName(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("Мерседес,-БМВ-и-Ауда.md", "# Мерседес, БМВ и Ауда\n\nРассказ"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("Груша.md", "Груша - фрукт"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("search_notes", []byte(`{"query":"мерседес"}`))
	if err != nil || res.Found == 0 || res.Hits[0].File != "Мерседес,-БМВ-и-Ауда.md" {
		t.Fatalf("мерседес = %+v, err = %v", res, err)
	}
	res, err = n.RunTool("search_notes", []byte(`{"query":"что я писал про мерседес"}`))
	if err != nil || res.Found == 0 || res.Hits[0].File != "Мерседес,-БМВ-и-Ауда.md" {
		t.Fatalf("user phrase = %+v, err = %v", res, err)
	}
	res, err = n.RunTool("search_notes", []byte(`{"query":"груша"}`))
	if err != nil || res.Found == 0 || res.Hits[0].File != "Груша.md" {
		t.Fatalf("груша = %+v, err = %v", res, err)
	}
}

func TestSearch_yoMatchesYe(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("oblast.md", "Городской ландшафт: Небоскрёбы + горные дороги"); err != nil {
		t.Fatal(err)
	}
	hits, err := n.Search("небоскребы")
	if err != nil || len(hits) != 1 || hits[0].File != "oblast.md" {
		t.Fatalf("небоскребы = %+v, err = %v", hits, err)
	}
}

func TestSearch_rebuildsAfterExternalDelete(t *testing.T) {
	dir := t.TempDir()
	n, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })
	if _, err := n.Save("keep.md", "keep word"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("gone.md", "gone word"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "gone.md")); err != nil {
		t.Fatal(err)
	}
	hits, err := n.Search("gone")
	if err != nil || len(hits) != 0 {
		t.Fatalf("stale hit = %+v, err = %v", hits, err)
	}
	keep, err := n.Search("keep")
	if err != nil || len(keep) != 1 || keep[0].File != "keep.md" {
		t.Fatalf("keep after delete = %+v, err = %v", keep, err)
	}
}

func TestSearch_matchesTitlePrefix(t *testing.T) {
	n := openTest(t)
	ref := "automotive-brand/REUS/03 — Design/References.md"
	other := "automotive-brand/REUS/02 — Vehicles/First Model.md"
	if _, err := n.Save(ref, "# References\n\nDesign refs"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save(other, "# First Model\n\nsee 03 — Design/References/First Model/image.png"); err != nil {
		t.Fatal(err)
	}

	hits, err := n.SearchUI("Refe")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].File != ref {
		t.Fatalf("Refe hits = %+v, err = %v", hits, err)
	}

	hits, err = n.SearchUI("References")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].File != ref {
		t.Fatalf("References hits = %+v, err = %v", hits, err)
	}
}
