package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func mountDrive(t *testing.T) (*http.ServeMux, *storage.Drive) {
	t.Helper()
	drive, err := storage.OpenDrive(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = drive.Close() })
	mux := http.NewServeMux()
	MountDrive(mux, drive)
	return mux, drive
}

func TestDriveAPI_mkdirList(t *testing.T) {
	mux, _ := mountDrive(t)
	rec := serveJSON(t, mux, http.MethodPost, "/api/drive/mkdir", `{"path":"photos/2024"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("mkdir status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("root list status = %d body = %s", rec.Code, rec.Body.String())
	}
	var root []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if len(root) != 1 || root[0].Path != "photos" || !root[0].IsDir {
		t.Fatalf("root = %+v", root)
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=photos", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", rec.Code, rec.Body.String())
	}
	var entries []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "photos/2024" || !entries[0].IsDir {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestDriveAPI_searchNested(t *testing.T) {
	mux, _ := mountDrive(t)
	rec := postDriveUpload(t, mux, "photos/2024", "cat.jpg", "xx")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?q=cat", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d body = %s", rec.Code, rec.Body.String())
	}
	var hits []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "cat.jpg" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestDriveAPI_rejectsTraversal(t *testing.T) {
	mux, _ := mountDrive(t)
	rec := serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=../etc", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("list traversal status = %d body = %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/drive/file/.secret", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("hidden file status = %d body = %s", rec.Code, rec.Body.String())
	}
	// ServeMux cleans ".." and may 307; cleaned path must not escape the drive.
	req = httptest.NewRequest(http.MethodGet, "/api/drive/file/../x", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("file traversal served body = %s", rec.Body.String())
	}
}

func TestDriveAPI_uploadAndStream(t *testing.T) {
	mux, _ := mountDrive(t)
	rec := postDriveUpload(t, mux, "docs", "hello.txt", "hello drive")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=docs", "")
	var entries []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "hello.txt" {
		t.Fatalf("list = %+v", entries)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/drive/file/docs/hello.txt", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stream status = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "hello drive" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Accept-Ranges"), "bytes") {
		t.Fatalf("Accept-Ranges = %q", rec.Header().Get("Accept-Ranges"))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/drive/file/docs/hello.txt", nil)
	req.Header.Set("Range", "bytes=0-4")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("range body = %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/drive/file/docs/missing.txt", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestDriveAPI_uploadNestedFolder(t *testing.T) {
	mux, _ := mountDrive(t)
	rec := postDriveUpload(t, mux, "docs", "album/2024/cat.jpg", "xx")
	if rec.Code != http.StatusOK {
		t.Fatalf("nested filename status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=docs", "")
	var top []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &top); err != nil {
		t.Fatal(err)
	}
	if len(top) != 1 || top[0].Path != "docs/album" || !top[0].IsDir {
		t.Fatalf("docs list = %+v", top)
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=docs/album/2024", "")
	var nested []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &nested); err != nil {
		t.Fatal(err)
	}
	if len(nested) != 1 || nested[0].Path != "docs/album/2024/cat.jpg" {
		t.Fatalf("nested list = %+v", nested)
	}

	rec = postDriveUploadRel(t, mux, "", "trip/note.txt", "hi")
	if rec.Code != http.StatusOK {
		t.Fatalf("rel status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=trip", "")
	var trip []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &trip); err != nil {
		t.Fatal(err)
	}
	if len(trip) != 1 || trip[0].Name != "note.txt" {
		t.Fatalf("trip list = %+v", trip)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("path", ""); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("rel", "mix/ok.txt"); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("file", "ok.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("rel", "mix/.DS_Store"); err != nil {
		t.Fatal(err)
	}
	part, err = w.CreateFormFile("file", ".DS_Store")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "skip"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/drive/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("skip hidden status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=mix", "")
	var mix []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &mix); err != nil {
		t.Fatal(err)
	}
	if len(mix) != 1 || mix[0].Name != "ok.txt" {
		t.Fatalf("mix list = %+v", mix)
	}

	var dirBuf bytes.Buffer
	dw := multipart.NewWriter(&dirBuf)
	if err := dw.WriteField("path", "docs"); err != nil {
		t.Fatal(err)
	}
	if err := dw.WriteField("dir", "empty/inner"); err != nil {
		t.Fatal(err)
	}
	if err := dw.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/drive/upload", &dirBuf)
	req.Header.Set("Content-Type", dw.FormDataContentType())
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty dir status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=docs/empty", "")
	var empty []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if len(empty) != 1 || empty[0].Path != "docs/empty/inner" || !empty[0].IsDir {
		t.Fatalf("empty list = %+v", empty)
	}
}

func TestSkipDriveUploadName(t *testing.T) {
	if skipDriveUploadName(".hidden") {
		t.Fatal("top-level hidden must not skip, so upload still 400")
	}
	if skipDriveUploadName("../x") {
		t.Fatal("traversal must not skip")
	}
	if !skipDriveUploadName("album/.DS_Store") {
		t.Fatal("nested hidden should skip")
	}
	if !skipDriveUploadName("x.part") {
		t.Fatal(".part should skip")
	}
	if skipDriveUploadName("album/cat.jpg") {
		t.Fatal("normal nested file")
	}
}

func TestDriveAPI_uploadRejectsBadName(t *testing.T) {
	mux, _ := mountDrive(t)
	rec := postDriveUpload(t, mux, "../x", "a.txt", "no")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("dest traversal status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = postDriveUpload(t, mux, "docs", "..", "no")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad filename status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = postDriveUpload(t, mux, "docs", ".hidden", "no")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("hidden filename status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestDriveAPI_deleteItem(t *testing.T) {
	mux, _ := mountDrive(t)
	rec := postDriveUpload(t, mux, "docs", "a.txt", "bye")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/drive/item/docs/a.txt", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveJSON(t, mux, http.MethodGet, "/api/drive/list?path=docs", "")
	var entries []storage.DriveEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("list after delete = %+v", entries)
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/drive/item/.secret", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("hidden delete status = %d", rec.Code)
	}
}

func TestCapReader(t *testing.T) {
	b, err := io.ReadAll(&capReader{r: strings.NewReader("hello"), max: 3})
	if !errors.Is(err, errDriveTooLarge) {
		t.Fatalf("err = %v data = %q", err, b)
	}
	b, err = io.ReadAll(&capReader{r: strings.NewReader("hello"), max: 5})
	if err != nil || string(b) != "hello" {
		t.Fatalf("exact = %q err = %v", b, err)
	}
}

func postDriveUploadRel(t *testing.T, mux http.Handler, dest, rel, body string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("path", dest); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("rel", rel); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("file", "ignored.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/drive/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func postDriveUpload(t *testing.T, mux http.Handler, dest, filename, body string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("path", dest); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/drive/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
