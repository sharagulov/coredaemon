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

const maxToolTurns = 8

var allowedTools = map[string]bool{
	"create_note":    true,
	"append_to_note": true,
	"read_note":      true,
	"search_notes":   true,
}

var blockedTools = map[string]bool{
	"trash_note":  true,
	"delete_note": true,
	"remove_note": true,
	"move_note":   true,
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
	Previous     map[string]string   `json:"previous,omitempty"`
	Searched     bool                `json:"searched,omitempty"`
	Matches      []storage.SearchHit `json:"matches,omitempty"`
	System       bool                `json:"system,omitempty"`
}

// Chat runs the event loop: model → tool calls → storage → model → final text.
func (a *Agent) Chat(ctx context.Context, userMessages []Message, progress ProgressFunc, scope string) (ChatResult, error) {
	scope = strings.TrimSpace(scope)
	messages := WithToolSystem(userMessages, a.scopeHint(scope))
	attachedNotes, attached := a.loadAttachedNotes(scope, userMessages)
	if attachedNotes != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: attachedNotes})
	}
	tools := NoteTools()
	notesChanged := false
	nudged := false
	searched := false
	blockedHit := false
	toolFailed := false
	found := 0
	total := 0
	listed := false
	var reads []readFact
	var createdFiles, updatedFiles []string
	createdSet := map[string]struct{}{}
	previous := map[string]string{}
	var matches []storage.SearchHit

	for turn := 0; turn < maxToolTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return ChatResult{}, err
		}
		emitProgress(progress, Phase{Kind: "thinking"})
		msg, err := a.client.ChatOnce(ctx, messages, tools)
		if err != nil {
			return ChatResult{}, err
		}
		msg = normalizeAssistant(msg)
		if names := toolCallNames(msg.ToolCalls); len(names) > 0 {
			log.Printf("agent: tool_calls %s", strings.Join(names, ", "))
		}

		if len(msg.ToolCalls) == 0 {
			content := strings.TrimSpace(msg.Content)
			if content == "" && !nudged {
				nudged = true
				nudge := AnswerNudge
				if toolFailed {
					nudge = ErrorNudge
				}
				messages = append(messages, Message{Role: RoleSystem, Content: nudge})
				continue
			}
			if content == "" {
				return ChatResult{}, errors.New("empty response from ollama")
			}
			out := groundedContent(content, groundArgs{
				searched:     searched,
				matches:      matches,
				found:        found,
				total:        total,
				listed:       listed,
				notesChanged: notesChanged,
				blocked:      blockedHit,
				reads:        reads,
				attached:     attached,
			})
			return ChatResult{
				Content:      out,
				NotesChanged: notesChanged,
				Created:      createdFiles,
				Updated:      updatedFiles,
				Previous:     previousOrNil(previous),
				Searched:     searched,
				Matches:      matches,
				System:       isSystemReply(out),
			}, nil
		}

		msg.Content = ""
		messages = append(messages, msg)

		for _, call := range msg.ToolCalls {
			if err := ctx.Err(); err != nil {
				return ChatResult{}, err
			}
			if call.Type != "" && call.Type != "function" {
				continue
			}
			name := call.Function.Name
			if blockedTools[name] {
				blockedHit = true
				toolFailed = true
				result := storage.ToolResult{Status: "error", Error: storage.BlockedMutationMsg}
				logToolResult(name, result)
				messages = append(messages, toolMessage(name, result))
				continue
			}
			if !allowedTools[name] {
				toolFailed = true
				result := storage.ToolResult{Status: "error", Error: "tool not allowed"}
				logToolResult(name, result)
				messages = append(messages, toolMessage(name, result))
				continue
			}

			result, err := a.notes.RunToolScoped(name, call.Function.Arguments, scope)
			if err != nil {
				return ChatResult{}, err
			}
			if result.Status != "success" {
				toolFailed = true
			}
			logToolResult(name, result)
			if result.Status == "success" && name == "read_note" {
				reads = append(reads, readFact{File: result.File, Content: result.Content})
			}
			if name == "search_notes" && result.Status == "success" {
				searched = true
				listed = result.Listed
				matches = result.Hits
				if matches == nil {
					matches = []storage.SearchHit{}
				}
				found = result.Found
				if found < len(matches) {
					found = len(matches)
				}
				total = result.Total
				if total < found {
					total = found
				}
			}
			if result.Status == "success" && result.File != "" {
				switch name {
				case "create_note":
					notesChanged = true
					createdFiles = append(createdFiles, result.File)
					createdSet[result.File] = struct{}{}
					emitProgress(progress, Phase{Kind: "created", File: result.File, Title: result.Title})
				case "append_to_note":
					notesChanged = true
					if result.NewFile {
						createdFiles = append(createdFiles, result.File)
						createdSet[result.File] = struct{}{}
						emitProgress(progress, Phase{Kind: "created", File: result.File, Title: result.Title})
						break
					}
					if _, created := createdSet[result.File]; !created {
						if _, seen := previous[result.File]; !seen {
							previous[result.File] = result.Previous
						}
					}
					updatedFiles = append(updatedFiles, result.File)
					prev := result.Previous
					emitProgress(progress, Phase{Kind: "updated", File: result.File, Title: result.Title, Previous: &prev})
				}
			}
			messages = append(messages, toolMessage(name, result))
		}
	}

	return ChatResult{}, fmt.Errorf("tool loop exceeded %d turns", maxToolTurns)
}

func (a *Agent) scopeHint(scope string) string {
	if scope == "" {
		return ""
	}
	if scope == "important" {
		return " Работай только с заметками из раздела «Важные»."
	}
	sections, err := a.notes.ListSections()
	if err != nil {
		return ""
	}
	for _, s := range sections {
		if s.ID == scope {
			return fmt.Sprintf(" Работай только с заметками из раздела «%s».", s.Name)
		}
	}
	return ""
}

func previousOrNil(previous map[string]string) map[string]string {
	if len(previous) == 0 {
		return nil
	}
	return previous
}

func isSystemReply(s string) bool {
	return s == storage.BlockedMutationMsg || s == EmptySearchReply
}

type readFact struct {
	File    string
	Content string
}

type groundArgs struct {
	searched     bool
	matches      []storage.SearchHit
	found        int
	total        int
	listed       bool
	notesChanged bool
	blocked      bool
	reads        []readFact
	attached     bool
}

func groundedContent(content string, g groundArgs) string {
	if g.blocked {
		return storage.BlockedMutationMsg
	}
	if g.notesChanged {
		return content
	}
	if g.attached || len(g.reads) > 0 {
		return content
	}
	if g.searched {
		if g.listed {
			return formatNoteList(g.matches, g.found)
		}
		if len(g.matches) == 0 {
			return EmptySearchReply
		}
		return formatSearchHits(g.matches, g.found, g.total)
	}
	return content
}

func formatNoteList(hits []storage.SearchHit, found int) string {
	if found < len(hits) {
		found = len(hits)
	}
	var b strings.Builder
	b.WriteString("Всего заметок: ")
	b.WriteString(fmt.Sprintf("%d", found))
	for _, h := range hits {
		b.WriteString("\n• ")
		b.WriteString(h.File)
	}
	return b.String()
}

func formatSearchHits(hits []storage.SearchHit, found, total int) string {
	if found < len(hits) {
		found = len(hits)
	}
	var b strings.Builder
	b.WriteString("Найдено: ")
	b.WriteString(fmt.Sprintf("%d", found))
	if total > found {
		b.WriteString(fmt.Sprintf(" из %d", total))
	}
	if found > len(hits) {
		b.WriteString(fmt.Sprintf(" (показаны %d)", len(hits)))
	}
	for _, h := range hits {
		b.WriteString("\n• ")
		b.WriteString(h.File)
	}
	return b.String()
}

func toolCallNames(calls []ToolCall) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		if c.Function.Name != "" {
			out = append(out, c.Function.Name)
		}
	}
	return out
}

func logToolResult(name string, result storage.ToolResult) {
	body, _ := json.Marshal(result)
	log.Printf("agent: %s -> %s", name, body)
}

func toolMessage(name string, result storage.ToolResult) Message {
	body, _ := json.Marshal(result)
	return Message{
		Role:     RoleTool,
		ToolName: name,
		Content:  string(body),
	}
}
