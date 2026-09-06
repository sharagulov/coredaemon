package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSection(t *testing.T) {
	n := openTest(t)

	sec, err := n.CreateSection("Работа")
	if err != nil {
		t.Fatal(err)
	}
	if sec.ID == "" || sec.Name != "Работа" || sec.CreatedAt == "" {
		t.Fatalf("section = %+v", sec)
	}

	list, err := n.ListSections()
	if err != nil || len(list) != 1 || list[0].ID != sec.ID {
		t.Fatalf("list = %+v, err = %v", list, err)
	}

	if _, err := n.CreateSection("Работа"); err != ErrSectionExists {
		t.Fatalf("duplicate err = %v", err)
	}
	if _, err := n.CreateSection(""); err != ErrSectionInvalid {
		t.Fatalf("empty err = %v", err)
	}
}

func TestCreateSection_rejectsPathNames(t *testing.T) {
	dir := t.TempDir()
	n, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })

	for _, name := range []string{"foo/bar", `foo\bar`, "../secret", "foo/../bar"} {
		if _, err := n.CreateSection(name); err != ErrSectionInvalid {
			t.Fatalf("CreateSection(%q) = %v", name, err)
		}
	}

	list, err := n.ListSections()
	if err != nil || len(list) != 0 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(dir, sectionsFileName)); !os.IsNotExist(err) {
		t.Fatalf("sections file should not be written: %v", err)
	}
}

func TestNoteSectionAndImportant(t *testing.T) {
	n := openTest(t)

	sec, err := n.CreateSection("Ideas")
	if err != nil {
		t.Fatal(err)
	}

	important := true
	note, err := n.SaveWithMeta("a.md", "hello", &NoteMetaInput{
		Section:   &sec.ID,
		Important: &important,
	})
	if err != nil {
		t.Fatal(err)
	}
	if note.Section != sec.ID || !note.Important {
		t.Fatalf("note = %+v", note)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	if list[0].Section != sec.ID || !list[0].Important {
		t.Fatalf("summary = %+v", list[0])
	}

	got, err := n.Get("a.md")
	if err != nil || got.Section != sec.ID || !got.Important {
		t.Fatalf("get = %+v, err = %v", got, err)
	}

	_, err = n.Save("a.md", "updated")
	if err != nil {
		t.Fatal(err)
	}
	got, err = n.Get("a.md")
	if err != nil || got.Section != sec.ID || !got.Important {
		t.Fatalf("preserve meta: %+v", got)
	}

	clear := ""
	note, err = n.UpdateMeta("a.md", NoteMetaInput{Section: &clear})
	if err != nil || note.Section != "" {
		t.Fatalf("clear section: %+v, err = %v", note, err)
	}
}

func TestDeleteSection(t *testing.T) {
	n := openTest(t)

	sec, err := n.CreateSection("Temp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.SaveWithMeta("a.md", "body", &NoteMetaInput{Section: &sec.ID}); err != nil {
		t.Fatal(err)
	}

	if err := n.DeleteSection(sec.ID); err != nil {
		t.Fatal(err)
	}
	list, err := n.ListSections()
	if err != nil || len(list) != 0 {
		t.Fatalf("sections = %+v, err = %v", list, err)
	}
	got, err := n.Get("a.md")
	if err != nil || got.Section != "" {
		t.Fatalf("note section = %q, err = %v", got.Section, err)
	}
	if err := n.DeleteSection(sec.ID); err != ErrSectionNotFound {
		t.Fatalf("delete again: %v", err)
	}
}

func TestValidateSectionID(t *testing.T) {
	n := openTest(t)
	bad := "missing"
	if err := n.validateSectionID(bad); err != ErrSectionNotFound {
		t.Fatalf("err = %v", err)
	}
	if err := n.validateSectionID("../secret"); err != ErrSectionInvalid {
		t.Fatalf("path err = %v", err)
	}
}
