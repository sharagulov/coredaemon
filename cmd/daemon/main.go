package main

import (
	"log"
	"net/http"
	"os"

	"github.com/core-daemon/core-daemon/internal/ai"
	"github.com/core-daemon/core-daemon/internal/api"
	"github.com/core-daemon/core-daemon/internal/storage"
	"github.com/core-daemon/core-daemon/internal/web"
)

func main() {
	notes, err := storage.Open("notes")
	if err != nil {
		log.Fatalf("notes: %v", err)
	}
	defer notes.Close()

	ollamaURL := envOr("OLLAMA_URL", "http://localhost:11434")
	ollamaModel := envOr("OLLAMA_MODEL", "qwen2.5:7b")
	llm := ai.New(ollamaURL, ollamaModel)
	agent := ai.NewAgent(llm, notes)

	mux := http.NewServeMux()
	api.MountNotes(mux, notes)
	api.MountChats(mux, notes)
	api.MountChat(mux, agent)
	web.Mount(mux)

	log.Printf("listening on :8080 (ollama %s, model %s)", ollamaURL, ollamaModel)
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
