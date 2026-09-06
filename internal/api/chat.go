package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/core-daemon/core-daemon/internal/ai"
)

const maxChatBody = 1 << 17 // 128 KiB

// MountChat registers POST /api/chat with SSE progress and a final result event.
func MountChat(mux *http.ServeMux, agent *ai.Agent) {
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxChatBody)
		defer r.Body.Close()

		var req struct {
			Message  string       `json:"message"`
			Messages []ai.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				writeErr(w, http.StatusBadRequest, "request body required")
				return
			}
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		messages := req.Messages
		if len(messages) == 0 {
			text := strings.TrimSpace(req.Message)
			if text == "" {
				writeErr(w, http.StatusBadRequest, "message is required")
				return
			}
			messages = []ai.Message{{Role: ai.RoleUser, Content: text}}
		}

		messages, err := normalizeChatMessages(messages)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}

		stream, err := newSSEWriter(w)
		if err != nil {
			log.Printf("chat: sse: %v", err)
			writeErr(w, http.StatusInternalServerError, "streaming unsupported")
			return
		}

		result, err := agent.Chat(r.Context(), messages, func(p ai.Phase) {
			if err := stream.Send("status", p); err != nil {
				log.Printf("chat: status stream: %v", err)
			}
		})
		if err != nil {
			log.Printf("chat: %v", err)
			_ = stream.Send("error", map[string]string{"error": chatErrorMessage(err)})
			return
		}

		if err := stream.Send("done", result); err != nil {
			log.Printf("chat: done stream: %v", err)
		}
	})
}

func normalizeChatMessages(in []ai.Message) ([]ai.Message, error) {
	if len(in) == 0 {
		return nil, errors.New("messages required")
	}
	if len(in) > 100 {
		return nil, errors.New("too many messages")
	}

	out := make([]ai.Message, 0, len(in))
	for _, m := range in {
		role := strings.TrimSpace(m.Role)
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		switch role {
		case ai.RoleUser, ai.RoleAssistant:
			out = append(out, ai.Message{Role: role, Content: content})
		default:
			return nil, errors.New("invalid message role")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("messages required")
	}
	if out[len(out)-1].Role != ai.RoleUser {
		return nil, errors.New("last message must be from user")
	}
	return out, nil
}

func chatErrorMessage(err error) string {
	if err == nil {
		return "неизвестная ошибка"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "запрос отменён"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "request ollama"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "no such host"):
		return "не удалось связаться с Ollama"
	case strings.Contains(msg, "empty response"):
		return "модель вернула пустой ответ"
	case strings.Contains(msg, "tool loop exceeded"):
		return "агент слишком долго вызывал инструменты"
	case strings.Contains(msg, "ollama status"):
		return "Ollama вернула ошибку"
	default:
		return "не удалось получить ответ"
	}
}
