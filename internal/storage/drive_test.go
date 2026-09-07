package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDrive_scanSeesExternalWrite(t *testing.T) {
	d := openDriveTest(t)
	nested := filepath.Join(d.root, "docs")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "a.pdf"), []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Scan(); err != nil {
		t.Fatal(err)
	}
	docs, err := d.List("docs")
	if err != nil || len(docs) != 1 || docs[0].Name != "a.pdf" || docs[0].Kind != "pdf" {
		t.Fatalf("docs = %+v, err=%v", docs, err)
	}
	if err := os.Remove(filepath.Join(nested, "a.pdf")); err != nil {
		t.Fatal(err)
	}
	if err := d.Scan(); err != nil {
		t.Fatal(err)
	}
	docs, err = d.List("docs")
	if err != nil || len(docs) != 0 {
		t.Fatalf("stale = %+v, err=%v", docs, err)
	}
}

func TestDrive_scanSkipsHiddenAndPart(t *testing.T) {
	d := openDriveTest(t)
	if err := os.WriteFile(filepath.Join(d.root, ".secret"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.root, "ok.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.root, "ok.txt.part"), []byte("tmp"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d.root, ".stash"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.root, ".stash", "in.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Scan(); err != nil {
		t.Fatal(err)
	}
	root, err := d.List("")
	if err != nil || len(root) != 1 || root[0].Name != "ok.txt" {
		t.Fatalf("root = %+v, err=%v", root, err)
	}
}

func TestDrive_saveFile(t *testing.T) {
	d := openDriveTest(t)
	if _, err := d.SaveFile("docs/a.txt", strings.NewReader("hi")); err != nil {
		t.Fatal(err)
	}
	docs, err := d.List("docs")
	if err != nil || len(docs) != 1 || docs[0].Name != "a.txt" || docs[0].Kind != "text" {
		t.Fatalf("docs = %+v, err=%v", docs, err)
	}
	if _, err := d.SaveFile("docs/a.txt", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(d.root, "docs", "a.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("overwrite = %q, err=%v", data, err)
	}
	if _, err := d.Mkdir("folder"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveFile("folder", strings.NewReader("no")); err != ErrInvalidName {
		t.Fatalf("save onto dir err = %v", err)
	}
}

func TestDrive_removeFileAndFolder(t *testing.T) {
	d := openDriveTest(t)
	if _, err := d.SaveFile("docs/a.txt", strings.NewReader("hi")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveFile("photos/2024/x.txt", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if err := d.Remove("docs/a.txt"); err != nil {
		t.Fatal(err)
	}
	docs, err := d.List("docs")
	if err != nil || len(docs) != 0 {
		t.Fatalf("docs after remove = %+v, err=%v", docs, err)
	}
	if _, err := os.Stat(filepath.Join(d.root, "docs", "a.txt")); !os.IsNotExist(err) {
		t.Fatal("file still on disk")
	}
	if err := d.Remove("photos"); err != nil {
		t.Fatal(err)
	}
	root, err := d.List("")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range root {
		if e.Path == "photos" || strings.HasPrefix(e.Path, "photos/") {
			t.Fatalf("stale after folder remove: %+v", root)
		}
	}
	if _, err := os.Stat(filepath.Join(d.root, "photos")); !os.IsNotExist(err) {
		t.Fatal("folder still on disk")
	}
	if err := d.Remove("../x"); err != ErrInvalidName {
		t.Fatalf("traversal remove err = %v", err)
	}
	if err := d.Remove(""); err != ErrInvalidName {
		t.Fatalf("empty remove err = %v", err)
	}
}

func openDriveTest(t *testing.T) *Drive {
	t.Helper()
	d, err := OpenDrive(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestNormalizeDrivePath(t *testing.T) {
	ok, err := normalizeDrivePath("photos/2024/cat.jpg")
	if err != nil || ok != "photos/2024/cat.jpg" {
		t.Fatalf("got %q, err=%v", ok, err)
	}
	root, err := normalizeDrivePath("")
	if err != nil || root != "" {
		t.Fatalf("root = %q, err=%v", root, err)
	}
	for _, bad := range []string{
		"../etc/passwd", "a/../b", "/abs", "foo/./bar", ".hidden", "a/.secret/b", `a\..\b`,
		"a<b", "foo|bar", strings.Repeat("a/", 25) + "x",
	} {
		if _, err := normalizeDrivePath(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestDriveKindFromName(t *testing.T) {
	if kind, mime := driveKind("x.jpg", false); kind != "image" || mime != "image/jpeg" {
		t.Fatalf("jpg = %s %s", kind, mime)
	}
	if kind, mime := driveKind("x.mp4", false); kind != "video" || mime != "video/mp4" {
		t.Fatalf("mp4 = %s %s", kind, mime)
	}
	if kind, _ := driveKind("x.pdf", false); kind != "pdf" {
		t.Fatalf("pdf = %s", kind)
	}
	if kind, mime := driveKind("dir", true); kind != "folder" || mime != "inode/directory" {
		t.Fatalf("dir = %s %s", kind, mime)
	}
}

func TestDrive_listAndMkdir(t *testing.T) {
	d := openDriveTest(t)
	if _, err := d.Mkdir("photos/2024"); err != nil {
		t.Fatal(err)
	}
	root, err := d.List("")
	if err != nil || len(root) != 1 || root[0].Path != "photos" || !root[0].IsDir {
		t.Fatalf("root = %+v, err=%v", root, err)
	}
	photos, err := d.List("photos")
	if err != nil || len(photos) != 1 || photos[0].Path != "photos/2024" {
		t.Fatalf("photos = %+v, err=%v", photos, err)
	}
	if _, err := d.Mkdir("../x"); err == nil {
		t.Fatal("traversal mkdir")
	}
	if _, err := d.Mkdir(""); err != ErrInvalidName {
		t.Fatalf("empty mkdir err = %v", err)
	}
}
