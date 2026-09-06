package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/core-daemon/core-daemon/internal/storage"
)

const maxChatsBody = 2 << 20

// MountChats registers GET/PUT /api/chats for conversation history.
func MountChats(mux *http.ServeMux, notes *storage.Notes) {
	mux.HandleFunc("GET /api/chats", func(w http.ResponseWriter, r *http.Request) {
		store, err := notes.LoadChats()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to load chats")
			return
		}
		writeJSON(w, http.StatusOK, store)
	})

	mux.HandleFunc("PUT /api/chats", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxChatsBody)
		defer r.Body.Close()

		var store storage.ChatStore
		if err := json.NewDecoder(r.Body).Decode(&store); err != nil {
			if errors.Is(err, io.EOF) {
				writeErr(w, http.StatusBadRequest, "request body required")
				return
			}
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if err := notes.SaveChats(store); err != nil {
			if errors.Is(err, storage.ErrInvalidChats) {
				writeErr(w, http.StatusBadRequest, "invalid chats")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to save chats")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
