package storage

import (
	"strings"
	"testing"
)

func TestNotes_SaveGetList(t *testing.T) {
	n := openTest(t)

	if _, err := n.Save("note1.md", "hello"); err != nil {
		t.Fatal(err)
	}

	got, err := n.Get("note1.md")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello" {
		t.Fatalf("content = %q", got.Content)
	}

	list, err := n.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Preview != "hello" || list[0].Title != "Note1" {
		t.Fatalf("list = %+v", list)
	}
}

func TestNotes_SaveOverwrites(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("a.md", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("a.md", "v2"); err != nil {
		t.Fatal(err)
	}
	got, err := n.Get("a.md")
	if err != nil || got.Content != "v2" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

func TestNotes_rejectsBadNames(t *testing.T) {
	n := openTest(t)
	for _, name := range []string{"../x.md", "foo/../../x.md", "x.txt", "", ".hidden.md"} {
		if _, err := n.Get(name); err != ErrInvalidName {
			t.Fatalf("Get(%q): err = %v", name, err)
		}
	}
}

func TestNotes_nested(t *testing.T) {
	n := openTest(t)
	name := "automotive-brand/REUS/01 — Brand/Identity.md"
	if _, err := n.Save(name, "# Identity\n\nbrand"); err != nil {
		t.Fatal(err)
	}

	got, err := n.Get(name)
	if err != nil || got.Name != name || !strings.Contains(got.Content, "brand") {
		t.Fatalf("got = %+v, err = %v", got, err)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 || list[0].Name != name || list[0].Title != "Identity" {
		t.Fatalf("list = %+v, err = %v", list, err)
	}

	hits, err := n.Search("brand")
	if err != nil || len(hits) != 1 || hits[0].File != name {
		t.Fatalf("hits = %+v, err = %v", hits, err)
	}
}

func TestNotes_notFound(t *testing.T) {
	n := openTest(t)
	_, err := n.Get("missing.md")
	if err != ErrNotFound {
		t.Fatalf("err = %v", err)
	}
}

func TestNotes_Rename(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("folder/old.md", "hello"); err != nil {
		t.Fatal(err)
	}
	got, err := n.Rename("folder/old.md", "Новое имя")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "folder/Новое-имя.md" {
		t.Fatalf("name = %q", got.Name)
	}
	if _, err := n.Get("folder/old.md"); err != ErrNotFound {
		t.Fatalf("old name still exists: %v", err)
	}
	same, err := n.Rename(got.Name, "Новое имя")
	if err != nil || same.Name != got.Name {
		t.Fatalf("idempotent rename: %+v %v", same, err)
	}
}

func TestDisplayTitle(t *testing.T) {
	if got := displayTitle("cactus.md", "# Кактусы\n\nтекст"); got != "Кактусы" {
		t.Fatalf("heading title = %q", got)
	}
	if got := displayTitle("актусы_1.md", "просто текст"); got != "Актусы 1" {
		t.Fatalf("filename title = %q", got)
	}
	if got := displayTitle("a/b/актусы_1.md", "просто текст"); got != "Актусы 1" {
		t.Fatalf("nested filename title = %q", got)
	}
}
