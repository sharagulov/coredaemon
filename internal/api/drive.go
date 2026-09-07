package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/core-daemon/core-daemon/internal/storage"
)

const maxDrivePart = 4 << 30

// MountDrive registers drive list/mkdir/upload/stream routes.
func MountDrive(mux *http.ServeMux, drive *storage.Drive) {
	mux.HandleFunc("GET /api/drive/list", func(w http.ResponseWriter, r *http.Request) {
		entries, err := drive.List(r.URL.Query().Get("path"))
		if err != nil {
			writeDriveErr(w, err)
			return
		}
		if entries == nil {
			entries = []storage.DriveEntry{}
		}
		writeJSON(w, http.StatusOK, entries)
	})

	mux.HandleFunc("POST /api/drive/mkdir", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		defer r.Body.Close()
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		entry, err := drive.Mkdir(req.Path)
		if err != nil {
			writeDriveErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, entry)
	})

	mux.HandleFunc("POST /api/drive/upload", func(w http.ResponseWriter, r *http.Request) {
		created, err := receiveDriveUpload(r, drive)
		if err != nil {
			if errors.Is(err, errDriveTooLarge) {
				writeErr(w, http.StatusRequestEntityTooLarge, "file too large")
				return
			}
			writeDriveErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, created)
	})

	mux.HandleFunc("DELETE /api/drive/item/{path...}", func(w http.ResponseWriter, r *http.Request) {
		if err := drive.Remove(r.PathValue("path")); err != nil {
			writeDriveErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/drive/file/{path...}", func(w http.ResponseWriter, r *http.Request) {
		rel := r.PathValue("path")
		f, entry, err := drive.OpenFile(rel)
		if err != nil {
			writeDriveErr(w, err)
			return
		}
		defer f.Close()
		mod := time.Unix(0, entry.MTime)
		w.Header().Set("Content-Type", entry.MIME)
		http.ServeContent(w, r, entry.Name, mod, f)
	})
}

var errDriveTooLarge = errors.New("drive file too large")

// capReader errors if the stream exceeds max, so SaveFile never renames the .part.
type capReader struct {
	r   io.Reader
	n   int64
	max int64
}

func (c *capReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.max {
		return n, errDriveTooLarge
	}
	return n, err
}

func receiveDriveUpload(r *http.Request, drive *storage.Drive) ([]storage.DriveEntry, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}
	destDir := ""
	out := make([]storage.DriveEntry, 0)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name := part.FormName()
		if name == "path" {
			b, err := io.ReadAll(io.LimitReader(part, 2048))
			_ = part.Close()
			if err != nil {
				return nil, err
			}
			destDir = strings.TrimSpace(string(b))
			continue
		}
		if name != "file" {
			_ = part.Close()
			continue
		}
		base := path.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
		if base == "." || base == ".." || base == "" || strings.HasPrefix(base, ".") {
			_ = part.Close()
			return nil, storage.ErrInvalidName
		}
		rel := base
		if destDir != "" {
			rel = strings.Trim(destDir, "/") + "/" + base
		}
		entry, err := drive.SaveFile(rel, &capReader{r: part, max: maxDrivePart})
		_ = part.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, *entry)
	}
	return out, nil
}

func writeDriveErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrInvalidName):
		writeErr(w, http.StatusBadRequest, "invalid path")
	case errors.Is(err, storage.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		writeErr(w, http.StatusInternalServerError, "drive error")
	}
}
