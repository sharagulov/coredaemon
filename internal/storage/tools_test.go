package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	items, err := n.ListTrash()
	if err != nil || len(items) != 1 || items[0].Name != name || items[0].Title != "Identity" {
		t.Fatalf("trash list = %+v, err = %v", items, err)
	}
	if items[0].DaysLeft < 29 || items[0].DaysLeft > 30 {
		t.Fatalf("days_left = %d", items[0].DaysLeft)
	}
	if _, err := os.Stat(filepath.Join(n.dir, "automotive-brand")); !os.IsNotExist(err) {
		t.Fatalf("expected empty folders pruned, err = %v", err)
	}

	restored, err := n.Restore(items[0].ID)
	if err != nil || restored.Name != name || !strings.Contains(restored.Content, "brand") {
		t.Fatalf("restore = %+v, err = %v", restored, err)
	}
	got, err := n.Get(name)
	if err != nil || !strings.Contains(got.Content, "brand") {
		t.Fatalf("get restored = %+v, err = %v", got, err)
	}
	empty, err := n.ListTrash()
	if err != nil || len(empty) != 0 {
		t.Fatalf("trash after restore = %+v, err = %v", empty, err)
	}
}

func TestNotes_Trash_restoreCollision(t *testing.T) {
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

	items, err := n.ListTrash()
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %+v, err = %v", items, err)
	}
	restored, err := n.Restore(items[0].ID)
	if err != nil || restored.Name != "tasks-2.md" || restored.Content != "v1" {
		t.Fatalf("restore = %+v, err = %v", restored, err)
	}
}

func TestNotes_Trash_migratesLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, trashDirName, "folder", "old.md")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("# Old\nhello"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })

	items, err := n.ListTrash()
	if err != nil || len(items) != 1 || items[0].Name != "folder/old.md" || items[0].Title != "Old" {
		t.Fatalf("items = %+v, err = %v", items, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy file still present: %v", err)
	}
}

func TestNotes_Trash_expiresAfter30Days(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("old.md", "gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Trash("old.md"); err != nil {
		t.Fatal(err)
	}

	items, err := n.ListTrash()
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %+v, err = %v", items, err)
	}

	meta := trashMeta{Name: "old.md", TrashedAt: time.Now().UTC().Add(-31 * 24 * time.Hour)}
	body, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(n.dir, trashDirName, items[0].ID, trashMetaFile), body, 0o644); err != nil {
		t.Fatal(err)
	}

	gone, err := n.ListTrash()
	if err != nil || len(gone) != 0 {
		t.Fatalf("expired trash = %+v, err = %v", gone, err)
	}
	if _, err := n.Restore(items[0].ID); err != ErrNotFound {
		t.Fatalf("restore expired: %v", err)
	}
}

func TestNotes_RunTool_trashNoteBlocked(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("gone.md", "# Gone\n"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("trash_note", []byte(`{"filename":"gone.md"}`))
	if err != nil || res.Status != "error" || res.Error != BlockedMutationMsg {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if _, err := n.Get("gone.md"); err != nil {
		t.Fatalf("note should remain: %v", err)
	}
}

func TestNotes_RunTool_rejectsUnknown(t *testing.T) {
	n := openTest(t)
	res, err := n.RunTool("delete_all", []byte(`{}`))
	if err != nil || res.Status != "error" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}
