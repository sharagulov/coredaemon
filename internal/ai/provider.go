package ai

import (
	"context"
	"errors"
	"strings"
)

const (
	ProviderOllama = "ollama"
	ProviderOpenAI = "openai"
)

// ErrOpenAIUnavailable is returned when the user asked for OpenAI but no key was configured.
var ErrOpenAIUnavailable = errors.New("openai not configured")

// ChatModel is one chat completion backend. Ollama keeps its own Client; OpenAI is extra.
type ChatModel interface {
	ChatOnce(ctx context.Context, messages []Message, tools []Tool) (Message, error)
}

// ModelProvider is one option shown in the chat model picker.
type ModelProvider struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Model     string `json:"model,omitempty"`
	Available bool   `json:"available"`
}

// ParseProvider maps the UI/API value to a known backend. Empty means local Ollama.
func ParseProvider(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", ProviderOllama:
		return ProviderOllama, nil
	case ProviderOpenAI:
		return ProviderOpenAI, nil
	default:
		return "", errors.New("unknown provider")
	}
}
