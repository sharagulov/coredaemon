package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/core-daemon/core-daemon/internal/storage"
)

const maxBody = storage.MaxNoteSize + 4096

// MountNotes registers note routes on mux.
func MountNotes(mux *http.ServeMux, notes *storage.Notes) {
	mux.HandleFunc("GET /api/notes", func(w http.ResponseWriter, r *http.Request) {
		list, err := notes.List()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list notes")
			return
		}
		if list == nil {
			list = []storage.NoteSummary{}
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("GET /api/notes/{name...}", func(w http.ResponseWriter, r *http.Request) {
		note, err := notes.Get(r.PathValue("name"))
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "note not found")
			return
		}
		if errors.Is(err, storage.ErrInvalidName) {
			writeErr(w, http.StatusBadRequest, "invalid note name")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read note")
			return
		}
		writeJSON(w, http.StatusOK, note)
	})

	mux.HandleFunc("POST /api/notes", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		defer r.Body.Close()

		var req struct {
			Name      string `json:"name"`
			Content   string `json:"content"`
			Section   string `json:"section"`
			Important *bool  `json:"important"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				writeErr(w, http.StatusBadRequest, "request body required")
				return
			}
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			writeErr(w, http.StatusBadRequest, "name is required")
			return
		}

		var patch *storage.NoteMetaInput
		if req.Section != "" || req.Important != nil {
			patch = &storage.NoteMetaInput{}
			if req.Section != "" {
				s := strings.TrimSpace(req.Section)
				patch.Section = &s
			}
			if req.Important != nil {
				patch.Important = req.Important
			}
		}

		var note *storage.Note
		var err error
		if patch != nil {
			note, err = notes.SaveWithMeta(req.Name, req.Content, patch)
		} else {
			note, err = notes.Save(req.Name, req.Content)
		}
		if errors.Is(err, storage.ErrInvalidName) {
			writeErr(w, http.StatusBadRequest, "invalid note name")
			return
		}
		if errors.Is(err, storage.ErrContentTooLarge) {
			writeErr(w, http.StatusBadRequest, "content too large")
			return
		}
		if errors.Is(err, storage.ErrSectionInvalid) || errors.Is(err, storage.ErrSectionNotFound) {
			writeErr(w, http.StatusBadRequest, "invalid section")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to save note")
			return
		}
		writeJSON(w, http.StatusCreated, note)
	})

	mux.HandleFunc("PATCH /api/notes/{name...}", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		defer r.Body.Close()

		var req struct {
			Section   *string `json:"section"`
			Important *bool   `json:"important"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				writeErr(w, http.StatusBadRequest, "request body required")
				return
			}
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if req.Section == nil && req.Important == nil {
			writeErr(w, http.StatusBadRequest, "nothing to update")
			return
		}

		note, err := notes.UpdateMeta(r.PathValue("name"), storage.NoteMetaInput{
			Section:   req.Section,
			Important: req.Important,
		})
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "note not found")
			return
		}
		if errors.Is(err, storage.ErrInvalidName) {
			writeErr(w, http.StatusBadRequest, "invalid note name")
			return
		}
		if errors.Is(err, storage.ErrSectionInvalid) || errors.Is(err, storage.ErrSectionNotFound) {
			writeErr(w, http.StatusBadRequest, "invalid section")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update note")
			return
		}
		writeJSON(w, http.StatusOK, note)
	})

	mux.HandleFunc("GET /api/sections", func(w http.ResponseWriter, r *http.Request) {
		list, err := notes.ListSections()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list sections")
			return
		}
		if list == nil {
			list = []storage.Section{}
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /api/sections", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		defer r.Body.Close()

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				writeErr(w, http.StatusBadRequest, "request body required")
				return
			}
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		sec, err := notes.CreateSection(strings.TrimSpace(req.Name))
		if errors.Is(err, storage.ErrSectionInvalid) {
			writeErr(w, http.StatusBadRequest, "invalid section name")
			return
		}
		if errors.Is(err, storage.ErrSectionExists) {
			writeErr(w, http.StatusConflict, "section already exists")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to create section")
			return
		}
		writeJSON(w, http.StatusCreated, sec)
	})

	mux.HandleFunc("GET /api/trash", func(w http.ResponseWriter, r *http.Request) {
		list, err := notes.ListTrash()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list trash")
			return
		}
		if list == nil {
			list = []storage.TrashItem{}
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /api/trash", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		defer r.Body.Close()

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				writeErr(w, http.StatusBadRequest, "request body required")
				return
			}
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		rel, err := notes.Trash(strings.TrimSpace(req.Name))
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "note not found")
			return
		}
		if errors.Is(err, storage.ErrInvalidName) {
			writeErr(w, http.StatusBadRequest, "invalid note name")
			return
		}
		if errors.Is(err, storage.ErrContentTooLarge) {
			writeErr(w, http.StatusBadRequest, "content too large")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to trash note")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": rel})
	})

	mux.HandleFunc("GET /api/trash/{id}", func(w http.ResponseWriter, r *http.Request) {
		item, err := notes.GetTrash(r.PathValue("id"))
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "note not found")
			return
		}
		if errors.Is(err, storage.ErrInvalidName) {
			writeErr(w, http.StatusBadRequest, "invalid trash id")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read trash")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("POST /api/trash/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		note, err := notes.Restore(r.PathValue("id"))
		if errors.Is(err, storage.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "note not found")
			return
		}
		if errors.Is(err, storage.ErrInvalidName) {
			writeErr(w, http.StatusBadRequest, "invalid trash id")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to restore note")
			return
		}
		writeJSON(w, http.StatusOK, note)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
