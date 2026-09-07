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
func (a *Agent) Chat(ctx context.Context, userMessages []Message, progress ProgressFunc, scope string, attachments ...string) (ChatResult, error) {
	scope = strings.TrimSpace(scope)
	if isVaultCountQuery(lastUserText(userMessages)) && len(attachments) == 0 {
		return ChatResult{Content: a.vaultCount(scope), System: true}, nil
	}
	userMessages = lastUserTurn(userMessages)
	messages := WithToolSystem(userMessages, a.scopeHint(scope))
	if count := a.vaultCount(scope); count != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: count})
	}
	attachedNotes, _ := a.loadAttachedNotes(scope, attachments)
	if attachedNotes != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: attachedNotes})
	}
	searchText, matches, searched := a.loadSearchContext(scope, lastUserText(userMessages))
	if searchText != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: searchText})
	}
	tools := NoteTools()
	notesChanged := false
	nudged := false
	blockedHit := false
	toolFailed := false
	readOK := false
	seenCalls := map[string]struct{}{}
	var createdFiles, updatedFiles []string
	createdSet := map[string]struct{}{}
	previous := map[string]string{}

	finish := func(content string, system bool) ChatResult {
		return ChatResult{
			Content:      content,
			NotesChanged: notesChanged,
			Created:      createdFiles,
			Updated:      updatedFiles,
			Previous:     previousOrNil(previous),
			Searched:     searched,
			Matches:      matches,
			System:       system,
		}
	}

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
			if notesChanged {
				return finish(writeFact(createdFiles, updatedFiles), true), nil
			}
			content := cleanReply(msg.Content)
			if content == "" && !nudged && !blockedHit && (readOK || len(matches) > 0 || !searched) {
				nudged = true
				nudge := AnswerNudge
				if toolFailed {
					nudge = ErrorNudge
				}
				messages = append(messages, Message{Role: RoleSystem, Content: nudge})
				continue
			}
			if blockedHit {
				return finish(storage.BlockedMutationMsg, true), nil
			}
			if searched && len(matches) == 0 && !readOK {
				return finish(EmptySearchMsg, true), nil
			}
			if content == "" {
				if fact := searchFact(matches); fact != "" {
					return finish(fact, true), nil
				}
				return ChatResult{}, errors.New("empty response from ollama")
			}
			if claimsMutation(content) {
				return finish(storage.BlockedMutationMsg, true), nil
			}
			if len(matches) > 0 && claimsNothingFound(content) {
				return finish(searchFact(matches), true), nil
			}
			return finish(content, false), nil
		}

		msg.Content = ""
		messages = append(messages, msg)
		repeated := false
		vault := a.vaultTotal(scope)

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
				logToolResult(name, result, vault)
				messages = append(messages, toolMessage(name, result, vault))
				continue
			}
			if !allowedTools[name] {
				toolFailed = true
				result := storage.ToolResult{Status: "error", Error: "tool not allowed"}
				logToolResult(name, result, vault)
				messages = append(messages, toolMessage(name, result, vault))
				continue
			}
			key := toolKey(name, call.Function.Arguments)
			if _, ok := seenCalls[key]; ok {
				repeated = true
				toolFailed = true
				result := storage.ToolResult{Status: "error", Error: RepeatCallMsg}
				logToolResult(name, result, vault)
				messages = append(messages, toolMessage(name, result, vault))
				continue
			}
			seenCalls[key] = struct{}{}

			result, err := a.notes.RunToolScoped(name, call.Function.Arguments, scope)
			if err != nil {
				return ChatResult{}, err
			}
			if result.Status != "success" {
				toolFailed = true
			}
			logToolResult(name, result, vault)
			if result.Status == "success" && name == "read_note" {
				readOK = true
			}
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
					if !result.NewFile {
						if _, created := createdSet[result.File]; !created {
							if _, seen := previous[result.File]; !seen {
								previous[result.File] = result.Previous
							}
						}
						updatedFiles = append(updatedFiles, result.File)
						prev := result.Previous
						emitProgress(progress, Phase{Kind: "updated", File: result.File, Title: result.Title, Previous: &prev})
						break
					}
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
			messages = append(messages, toolMessage(name, result, vault))
		}
		if repeated {
			if notesChanged {
				return finish(writeFact(createdFiles, updatedFiles), true), nil
			}
			if nudged {
				if count := a.vaultCount(scope); count != "" {
					return finish(count, true), nil
				}
			}
			tools = nil
			if !nudged {
				nudged = true
				messages = append(messages, Message{Role: RoleSystem, Content: AnswerNudge})
			}
		}
	}

	if notesChanged {
		return finish(writeFact(createdFiles, updatedFiles), true), nil
	}
	if count := a.vaultCount(scope); count != "" {
		return finish(count, true), nil
	}
	return ChatResult{}, fmt.Errorf("tool loop exceeded %d turns", maxToolTurns)
}

func (a *Agent) scopeHint(scope string) string {
	if scope == "" {
		return ""
	}
	if scope == "important" {
		return "\nРаботай только с заметками из раздела «Важные»."
	}
	sections, err := a.notes.ListSections()
	if err != nil {
		return ""
	}
	for _, s := range sections {
		if s.ID == scope {
			return fmt.Sprintf("\nРаботай только с заметками из раздела «%s».", s.Name)
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

func toolCallNames(calls []ToolCall) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		if c.Function.Name != "" {
			out = append(out, c.Function.Name)
		}
	}
	return out
}

func logToolResult(name string, result storage.ToolResult, vault int) {
	log.Printf("agent: %s -> %s", name, encodeToolResult(name, result, vault))
}

func toolMessage(name string, result storage.ToolResult, vault int) Message {
	return Message{
		Role:     RoleTool,
		ToolName: name,
		Content:  string(encodeToolResult(name, result, vault)),
	}
}

func encodeToolResult(name string, result storage.ToolResult, vault int) []byte {
	if name == "search_notes" && result.Status == "success" {
		hits := result.Hits
		if hits == nil {
			hits = []storage.SearchHit{}
		}
		body, _ := json.Marshal(struct {
			Status string              `json:"status"`
			Hits   []storage.SearchHit `json:"hits"`
			Found  int                 `json:"found"`
			Vault  int                 `json:"vault"`
			Query  string              `json:"query,omitempty"`
		}{Status: result.Status, Hits: hits, Found: result.Found, Vault: vault, Query: result.Query})
		return body
	}
	if name == "read_note" && result.Status == "success" {
		result.Content = clipNote(result.Content)
	}
	body, _ := json.Marshal(result)
	return body
}

func toolKey(name string, args json.RawMessage) string {
	raw := strings.TrimSpace(string(args))
	if raw == "" || raw == "null" {
		raw = "{}"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return name + " " + raw
	}
	if name == "search_notes" {
		q, _ := m["query"].(string)
		q = strings.TrimSpace(q)
		if q == "*" {
			q = ""
		}
		body, _ := json.Marshal(map[string]string{"query": q})
		return name + " " + string(body)
	}
	body, _ := json.Marshal(m)
	return name + " " + string(body)
}
