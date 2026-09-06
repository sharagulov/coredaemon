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
	if created.Content != "- buy milk" {
		t.Fatalf("content = %q", created.Content)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 || list[0].Title != "Tasks" {
		t.Fatalf("list = %+v, err = %v", list, err)
	}

	cactus, err := n.CreateNote("яблоко", "фрукт")
	if err != nil {
		t.Fatal(err)
	}
	if cactus.Content != "фрукт" {
		t.Fatalf("content = %q", cactus.Content)
	}

	res, err := n.RunTool("read_note", []byte(`{"filename":"`+created.Name+`"}`))
	if err != nil || res.Status != "success" || res.Content == "" {
		t.Fatalf("read = %+v, err = %v", res, err)
	}

	res, err = n.RunTool("append_to_note", []byte(`{"filename":"`+created.Name+`","content":"- call mom"}`))
	if err != nil || res.Status != "success" || res.NewFile || res.Previous != created.Content {
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

func TestCreateNote_stripsDuplicateTitle(t *testing.T) {
	n := openTest(t)

	note, err := n.CreateNote("Новая заметка", "# Новая заметка\n\nТекст текст текст")
	if err != nil {
		t.Fatal(err)
	}
	if note.Content != "Текст текст текст" {
		t.Fatalf("content = %q", note.Content)
	}

	kept, err := n.CreateNote("Проект", "# Задачи\n\nсделать")
	if err != nil {
		t.Fatal(err)
	}
	if kept.Content != "# Задачи\n\nсделать" {
		t.Fatalf("kept other heading = %q", kept.Content)
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

func TestNotes_RunTool_appendMissingIsNew(t *testing.T) {
	n := openTest(t)
	res, err := n.RunTool("append_to_note", []byte(`{"filename":"fresh.md","content":"hello"}`))
	if err != nil || res.Status != "success" || !res.NewFile || res.File != "fresh.md" || res.Previous != "" {
		t.Fatalf("append missing = %+v, err = %v", res, err)
	}
}

func TestRunTool_appendResolvesExistingName(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("Мышь-полевая.md", "первая строка"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("append_to_note", []byte(`{"filename":"мышь полевая","content":"вторая строка"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || res.File != "Мышь-полевая.md" || res.NewFile {
		t.Fatalf("res = %+v", res)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	note, err := n.Get("Мышь-полевая.md")
	if err != nil || !strings.Contains(note.Content, "первая строка") || !strings.Contains(note.Content, "вторая строка") {
		t.Fatalf("note = %+v, err = %v", note, err)
	}
}

func TestRunTool_appendStillCreatesUnknownNote(t *testing.T) {
	n := openTest(t)
	res, err := n.RunTool("append_to_note", []byte(`{"filename":"Совсем-новая.md","content":"текст"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || !res.NewFile || res.File != "Совсем-новая.md" {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunTool_appendResolvesWordOrder(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("Мышь-полевая.md", "первая строка"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("append_to_note", []byte(`{"filename":"Полевая-мышь.md","content":"вторая строка"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || res.File != "Мышь-полевая.md" || res.NewFile {
		t.Fatalf("res = %+v", res)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	note, err := n.Get("Мышь-полевая.md")
	if err != nil || !strings.Contains(note.Content, "первая строка") || !strings.Contains(note.Content, "вторая строка") {
		t.Fatalf("note = %+v, err = %v", note, err)
	}
}

func TestRunTool_createSameTitleGetsUniqueName(t *testing.T) {
	n := openTest(t)
	first, err := n.RunTool("create_note", []byte(`{"title":"Мышь полевая","content":"одна"}`))
	if err != nil || first.Status != "success" || first.File != "Мышь-полевая.md" || !first.NewFile {
		t.Fatalf("first = %+v, err = %v", first, err)
	}
	second, err := n.RunTool("create_note", []byte(`{"title":"Мышь полевая","content":"две"}`))
	if err != nil || second.Status != "success" || second.File != "Мышь-полевая-2.md" || !second.NewFile {
		t.Fatalf("second = %+v, err = %v", second, err)
	}
}

func TestRunTool_createAppendsPermutedTitle(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("Мышь-полевая.md", "первая строка"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("create_note", []byte(`{"title":"Полевая мышь","content":"она рыжая"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || res.File != "Мышь-полевая.md" || res.NewFile {
		t.Fatalf("res = %+v", res)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	note, err := n.Get("Мышь-полевая.md")
	if err != nil || !strings.Contains(note.Content, "первая строка") || !strings.Contains(note.Content, "она рыжая") {
		t.Fatalf("note = %+v, err = %v", note, err)
	}
}

func TestNotes_RevertChanges_restoreAndTrash(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("keep.md", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("keep.md", "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Save("gone.md", "new"); err != nil {
		t.Fatal(err)
	}

	if err := n.RevertChanges([]string{"gone.md"}, map[string]string{"keep.md": "v1"}); err != nil {
		t.Fatal(err)
	}

	got, err := n.Get("keep.md")
	if err != nil || got.Content != "v1" {
		t.Fatalf("keep = %+v, err = %v", got, err)
	}
	if _, err := n.Get("gone.md"); err != ErrNotFound {
		t.Fatalf("gone should be trashed: %v", err)
	}
	items, err := n.ListTrash()
	if err != nil || len(items) != 1 || items[0].Name != "gone.md" {
		t.Fatalf("trash = %+v, err = %v", items, err)
	}
}

func TestNotes_RevertChanges_createdWins(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("only.md", "after"); err != nil {
		t.Fatal(err)
	}
	if err := n.RevertChanges([]string{"only.md"}, map[string]string{"only.md": "before"}); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Get("only.md"); err != ErrNotFound {
		t.Fatalf("created file should be trashed, not restored: %v", err)
	}
}
