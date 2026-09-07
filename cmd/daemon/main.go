package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/core-daemon/core-daemon/internal/ai"
	"github.com/core-daemon/core-daemon/internal/api"
	"github.com/core-daemon/core-daemon/internal/storage"
	"github.com/core-daemon/core-daemon/internal/web"
)

func main() {
	loadDotEnv(".env")

	notes, err := storage.Open("notes")
	if err != nil {
		log.Fatalf("notes: %v", err)
	}
	defer notes.Close()

	drive, err := storage.OpenDrive("data/drive")
	if err != nil {
		log.Fatalf("drive: %v", err)
	}
	defer drive.Close()
	drive.StartBackgroundScan()

	ollamaURL := envOr("OLLAMA_URL", "http://localhost:11434")
	ollamaModel := envOr("OLLAMA_MODEL", "qwen2.5-coder:14b")
	llm := ai.New(ollamaURL, ollamaModel)
	agent := ai.NewAgent(llm, notes)
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		openaiModel := envOr("OPENAI_MODEL", "gpt-4o-mini")
		agent.UseOpenAI(ai.NewOpenAI(envOr("OPENAI_URL", "https://api.openai.com/v1"), openaiModel, key), openaiModel)
		log.Printf("openai enabled, model %s", openaiModel)
	}

	mux := http.NewServeMux()
	api.MountNotes(mux, notes)
	api.MountChats(mux, notes)
	api.MountModels(mux, agent)
	api.MountChat(mux, agent)
	api.MountDrive(mux, drive)
	web.Mount(mux)

	log.Printf("listening on :8080 (ollama %s, model %s, drive data/drive)", ollamaURL, ollamaModel)
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
