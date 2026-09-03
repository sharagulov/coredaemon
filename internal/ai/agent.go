package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/core-daemon/core-daemon/internal/storage"
)

const (
	maxToolTurns = 8
	answerNudge  = "Ответь пользователю по результатам инструментов обычным текстом. " +
		"Не пиши XML и не повторяй вызовы, если результат уже получен."
)

var allowedTools = map[string]bool{
	"create_note":    true,
	"append_to_note": true,
	"read_note":      true,
	"search_notes":   true,
	"trash_note":     true,
}

// Agent runs the tool-calling loop against Ollama and storage.
type Agent struct {
	client *Client
	notes  *storage.Notes
}

// NewAgent creates an agent backed by client and notes storage.
func NewAgent(client *Client, notes *storage.Notes) *Agent {
	return &Agent{client: client, notes: notes}
}

// ChatResult is the final response to the browser.
type ChatResult struct {
	Content      string              `json:"content"`
	NotesChanged bool                `json:"notes_changed"`
	Created      []string            `json:"created,omitempty"`
	Updated      []string            `json:"updated,omitempty"`
	Trashed      []string            `json:"trashed,omitempty"`
	Searched     bool                `json:"searched,omitempty"`
	Matches      []storage.SearchHit `json:"matches,omitempty"`
}

// Chat runs the event loop: model → tool calls → storage → model → final text.
func (a *Agent) Chat(ctx context.Context, userMessages []Message, progress ProgressFunc) (ChatResult, error) {
	messages := WithToolSystem(userMessages)
	tools := NoteTools()
	notesChanged := false
	nudged := false
	searched := false
	var createdFiles, updatedFiles, trashedFiles []string
	var matches []storage.SearchHit

	for turn := 0; turn < maxToolTurns; turn++ {
		emitProgress(progress, Phase{Kind: "thinking"})
		msg, err := a.client.ChatOnce(ctx, messages, tools)
		if err != nil {
			return ChatResult{}, err
		}
		msg = normalizeAssistant(msg)

		if len(msg.ToolCalls) == 0 {
			content := strings.TrimSpace(msg.Content)
			if content == "" && !nudged {
				nudged = true
				messages = append(messages, Message{Role: RoleUser, Content: answerNudge})
				continue
			}
			if content == "" {
				return ChatResult{}, errors.New("empty response from ollama")
			}
			return ChatResult{
				Content:      groundedContent(content, searched, matches, notesChanged),
				NotesChanged: notesChanged,
				Created:      createdFiles,
				Updated:      updatedFiles,
				Trashed:      trashedFiles,
				Searched:     searched,
				Matches:      matches,
			}, nil
		}

		msg.Content = ""
		messages = append(messages, msg)

		for _, call := range msg.ToolCalls {
			if call.Type != "" && call.Type != "function" {
				continue
			}
			name := call.Function.Name
			if !allowedTools[name] {
				messages = append(messages, toolMessage(name, storage.ToolResult{
					Status: "error",
					Error:  "tool not allowed",
				}))
				continue
			}

			result, err := a.notes.RunTool(name, call.Function.Arguments)
			if err != nil {
				return ChatResult{}, err
			}
			log.Printf("agent: %s -> %s", name, result.Status)
			if name == "search_notes" && result.Status == "success" {
				searched = true
				matches = result.Hits
				if matches == nil {
					matches = []storage.SearchHit{}
				}
			}
			if result.Status == "success" && result.File != "" {
				switch name {
				case "create_note":
					notesChanged = true
					createdFiles = append(createdFiles, result.File)
					emitProgress(progress, Phase{Kind: "created", File: result.File, Title: result.Title})
				case "append_to_note":
					notesChanged = true
					updatedFiles = append(updatedFiles, result.File)
					emitProgress(progress, Phase{Kind: "updated", File: result.File, Title: result.Title})
				case "trash_note":
					notesChanged = true
					trashedFiles = append(trashedFiles, result.File)
					emitProgress(progress, Phase{Kind: "trashed", File: result.File, Title: result.Title})
				}
			}
			messages = append(messages, toolMessage(name, result))
		}
	}

	return ChatResult{}, fmt.Errorf("tool loop exceeded %d turns", maxToolTurns)
}

func groundedContent(content string, searched bool, matches []storage.SearchHit, notesChanged bool) string {
	if searched && len(matches) == 0 && !notesChanged {
		return "По этому запросу в заметках ничего не найдено."
	}
	return content
}

func toolMessage(name string, result storage.ToolResult) Message {
	body, _ := json.Marshal(result)
	return Message{
		Role:     RoleTool,
		ToolName: name,
		Content:  string(body),
	}
}
