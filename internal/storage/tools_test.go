package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotes_CreateAppendReadTool(t *testing.T) {
	n := openTest(t)

	created, err := n.CreateNote("Tasks", "- buy milk")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Content, "# Tasks\n") {
		t.Fatalf("expected heading, got %q", created.Content)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 || list[0].Title != "Tasks" {
		t.Fatalf("list = %+v, err = %v", list, err)
	}

	cactus, err := n.CreateNote("яблоко", "фрукт")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cactus.Content, "# Яблоко\n") {
		t.Fatalf("capitalized heading = %q", cactus.Content)
	}

	res, err := n.RunTool("read_note", []byte(`{"filename":"`+created.Name+`"}`))
	if err != nil || res.Status != "success" || res.Content == "" {
		t.Fatalf("read = %+v, err = %v", res, err)
	}

	res, err = n.RunTool("append_to_note", []byte(`{"filename":"`+created.Name+`","content":"- call mom"}`))
	if err != nil || res.Status != "success" {
		t.Fatalf("append = %+v, err = %v", res, err)
	}

	got, err := n.Get(created.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "call mom") {
		t.Fatalf("content = %q", got.Content)
	}
}

func TestNotes_Trash(t *testing.T) {
	n := openTest(t)
	name := "automotive-brand/REUS/01 — Brand/Identity.md"
	if _, err := n.Save(name, "# Identity\n\nbrand"); err != nil {
		t.Fatal(err)
	}

	rel, err := n.Trash(name)
	if err != nil || rel != name {
		t.Fatalf("trash = %q, err = %v", rel, err)
	}
	if _, err := n.Get(name); err != ErrNotFound {
		t.Fatalf("Get after trash: %v", err)
	}

	list, err := n.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	hits, err := n.Search("brand")
	if err != nil || len(hits) != 0 {
		t.Fatalf("search after trash = %+v, err = %v", hits, err)
	}

	trashed := filepath.Join(n.dir, trashDirName, filepath.FromSlash(name))
	data, err := os.ReadFile(trashed)
	if err != nil || !strings.Contains(string(data), "brand") {
		t.Fatalf("trash file = %q, err = %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(n.dir, "automotive-brand")); !os.IsNotExist(err) {
		t.Fatalf("expected empty folders pruned, err = %v", err)
	}
}

func TestNotes_Trash_collision(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("tasks.md", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Trash("tasks.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("tasks.md", "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Trash("tasks.md"); err != nil {
		t.Fatal(err)
	}

	first, err := os.ReadFile(filepath.Join(n.dir, trashDirName, "tasks.md"))
	if err != nil || string(first) != "v1" {
		t.Fatalf("first = %q, err = %v", first, err)
	}
	second, err := os.ReadFile(filepath.Join(n.dir, trashDirName, "tasks-2.md"))
	if err != nil || string(second) != "v2" {
		t.Fatalf("second = %q, err = %v", second, err)
	}
}

func TestNotes_RunTool_trashNote(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("gone.md", "# Gone\n"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("trash_note", []byte(`{"filename":"gone.md"}`))
	if err != nil || res.Status != "success" || res.File != "gone.md" || res.Title != "Gone" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}

	missing, err := n.RunTool("trash_note", []byte(`{"filename":"gone.md"}`))
	if err != nil || missing.Status != "error" {
		t.Fatalf("missing = %+v, err = %v", missing, err)
	}
}

func TestNotes_RunTool_rejectsUnknown(t *testing.T) {
	n := openTest(t)
	res, err := n.RunTool("delete_all", []byte(`{}`))
	if err != nil || res.Status != "error" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}
