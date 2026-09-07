package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html style.css app.js cm-bundle.js ui/*.js ui/chat/*.js ui/drive/*.js assets/*
var Files embed.FS

// Mount registers static assets and SPA routes on mux.
func Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes", serveIndex)
	mux.HandleFunc("GET /drive", serveIndex)
	mux.Handle("/", http.FileServer(http.FS(Files)))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(Files, "index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
