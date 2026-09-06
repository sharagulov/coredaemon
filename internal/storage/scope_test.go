package storage

import "testing"

func TestNoteMatchesScope(t *testing.T) {
	if !NoteMatchesScope("work", false, "") {
		t.Fatal("empty scope should match any note")
	}
	if !NoteMatchesScope("", true, "important") {
		t.Fatal("important scope should match important note")
	}
	if NoteMatchesScope("work", false, "important") {
		t.Fatal("important scope should not match section note")
	}
	if !NoteMatchesScope("work", false, "work") {
		t.Fatal("section scope should match same section")
	}
	if NoteMatchesScope("work", true, "work") {
		t.Fatal("section scope should not match important note")
	}
}

func TestRunToolScoped_filtersSearch(t *testing.T) {
	n := openTest(t)
	sec, err := n.CreateSection("Work")
	if err != nil {
		t.Fatal(err)
	}
	important := true
	if _, err := n.SaveWithMeta("imp.md", "ball", &NoteMetaInput{Important: &important}); err != nil {
		t.Fatal(err)
	}
	if _, err := n.SaveWithMeta("other/work.md", "ball", &NoteMetaInput{Section: &sec.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("plain.md", "ball"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunToolScoped("search_notes", []byte(`{"query":"ball"}`), "important")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || res.Found != 1 || len(res.Hits) != 1 || res.Hits[0].File != "imp.md" {
		t.Fatalf("important hits = %+v", res)
	}

	res, err = n.RunToolScoped("search_notes", []byte(`{"query":"ball"}`), sec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Found != 1 || res.Hits[0].File != "other/work.md" {
		t.Fatalf("section hits = %+v", res)
	}
}

func TestRunToolScoped_blocksReadOutsideScope(t *testing.T) {
	n := openTest(t)
	note, err := n.Save("secret.md", "body")
	if err != nil {
		t.Fatal(err)
	}
	res, err := n.RunToolScoped("read_note", []byte(`{"filename":"`+note.Name+`"}`), "important")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "error" || res.Error != "note not in scope" {
		t.Fatalf("read = %+v", res)
	}
}

func strPtr(s string) *string { return &s }
