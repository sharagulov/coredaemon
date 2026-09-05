package storage

import (
	"strings"
	"testing"
	"time"
)

func TestParseFrontmatter(t *testing.T) {
	raw := "---\ncreated: 2026-01-02T10:00:00Z\nmodified: 2026-01-03T12:00:00Z\nsection: work\nimportant: true\n---\n# Title\n\nbody"
	fields, body, ok := parseFrontmatter(raw)
	if !ok {
		t.Fatal("expected frontmatter")
	}
	if fields.Created.Format(time.RFC3339) != "2026-01-02T10:00:00Z" {
		t.Fatalf("created = %v", fields.Created)
	}
	if fields.Modified.Format(time.RFC3339) != "2026-01-03T12:00:00Z" {
		t.Fatalf("modified = %v", fields.Modified)
	}
	if fields.Section != "work" || !fields.Important {
		t.Fatalf("fields = %+v", fields)
	}
	if !strings.HasPrefix(body, "# Title") {
		t.Fatalf("body = %q", body)
	}
}

func TestNoteBody_withoutFrontmatter(t *testing.T) {
	if got := noteBody("hello"); got != "hello" {
		t.Fatalf("body = %q", got)
	}
}

func TestNotes_SavePreservesCreated(t *testing.T) {
	n := openTest(t)

	first, err := n.Save("a.md", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if first.CreatedAt == "" || first.ModifiedAt == "" {
		t.Fatalf("timestamps missing: %+v", first)
	}

	time.Sleep(1100 * time.Millisecond)

	second, err := n.Save("a.md", "v2")
	if err != nil {
		t.Fatal(err)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("created changed: %q -> %q", first.CreatedAt, second.CreatedAt)
	}
	if second.ModifiedAt == first.ModifiedAt {
		t.Fatalf("modified should change")
	}

	onDisk, err := n.Get("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Content != "v2" {
		t.Fatalf("content = %q", onDisk.Content)
	}
	if !strings.HasPrefix(onDisk.Content, "---") {
		// body returned without frontmatter
	} else {
		t.Fatal("Get should strip frontmatter from content")
	}
}
