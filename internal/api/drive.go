package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/core-daemon/core-daemon/internal/storage"
)

const maxDrivePart = 4 << 30

// MountDrive registers drive list/mkdir/upload/stream routes.
func MountDrive(mux *http.ServeMux, drive *storage.Drive) {
	mux.HandleFunc("GET /api/drive/list", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		var entries []storage.DriveEntry
		var err error
		if q == "" {
			entries, err = drive.List(r.URL.Query().Get("path"))
		} else {
			entries, err = drive.Search(r.URL.Query().Get("path"), q)
		}
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
	pendingRel := ""
	out := make([]storage.DriveEntry, 0)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch part.FormName() {
		case "path":
			b, err := io.ReadAll(io.LimitReader(part, 2048))
			_ = part.Close()
			if err != nil {
				return nil, err
			}
			destDir = strings.TrimSpace(string(b))
		case "rel":
			b, err := io.ReadAll(io.LimitReader(part, 2048))
			_ = part.Close()
			if err != nil {
				return nil, err
			}
			pendingRel = strings.TrimSpace(string(b))
		case "dir":
			b, err := io.ReadAll(io.LimitReader(part, 2048))
			_ = part.Close()
			if err != nil {
				return nil, err
			}
			raw := strings.TrimSpace(string(b))
			if skipDriveUploadName(raw) {
				continue
			}
			if _, err := drive.Mkdir(driveUploadDest(destDir, raw)); err != nil {
				return nil, err
			}
		case "file":
			raw := pendingRel
			pendingRel = ""
			if raw == "" {
				raw = partUploadName(part)
			}
			if skipDriveUploadName(raw) {
				drainPart(part)
				continue
			}
			rel := driveUploadDest(destDir, raw)
			entry, err := drive.SaveFile(rel, &capReader{r: part, max: maxDrivePart})
			_ = part.Close()
			if err != nil {
				return nil, err
			}
			out = append(out, *entry)
		default:
			_ = part.Close()
		}
	}
	return out, nil
}

func partUploadName(part *multipart.Part) string {
	disp := part.Header.Get("Content-Disposition")
	if i := strings.Index(disp, ";"); i >= 0 {
		_, params, err := mime.ParseMediaType("x/x" + disp[i:])
		if err == nil {
			if name := params["filename"]; name != "" {
				return name
			}
		}
	}
	return part.FileName()
}

func driveUploadDest(destDir, name string) string {
	name = strings.Trim(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"), "/")
	destDir = strings.Trim(strings.ReplaceAll(strings.TrimSpace(destDir), "\\", "/"), "/")
	if destDir == "" {
		return name
	}
	if name == "" {
		return destDir
	}
	return destDir + "/" + name
}

func skipDriveUploadName(name string) bool {
	name = strings.Trim(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"), "/")
	if name == "" {
		return false
	}
	parts := strings.Split(name, "/")
	for _, p := range parts {
		if p == ".." {
			return false
		}
	}
	nested := len(parts) > 1
	for _, p := range parts {
		if p == "." || p == "" {
			return nested
		}
		if strings.HasSuffix(p, ".part") {
			return true
		}
		if strings.HasPrefix(p, ".") {
			return nested
		}
	}
	return false
}

func drainPart(part *multipart.Part) {
	_, _ = io.Copy(io.Discard, io.LimitReader(part, maxDrivePart))
	_ = part.Close()
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
