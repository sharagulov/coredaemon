package ai

import (
	"encoding/json"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"

	SystemPrompt = "Ты полезный ассистент. Всегда отвечай на русском языке. " +
		"Отвечай точно, без выдуманных фактов. Не переключайся на другие языки."

	// ToolSystemPrompt is the system instruction for tool-enabled chat.
	ToolSystemPrompt = SystemPrompt +
		" Файлы на диске меняются только через create_note, append_to_note и trash_note. " +
		"trash_note переносит заметку в корзину, не удаляет навсегда. " +
		"search_notes ищет по всем заметкам — вызови его, если вопрос про уже записанное. " +
		"Если search_notes вернул found 0, скажи что ничего не найдено. Не выдумывай названия заметок. " +
		"Упоминай только file и title из hits. " +
		"read_note читает одну заметку целиком. " +
		"Никогда не сообщай о создании, изменении или переносе в корзину, если инструмент не вернул status success. " +
		"Если trash_note вернул success, скажи что заметка в корзине, не что удалена навсегда. " +
		"Каждая отдельная заметка — отдельный вызов create_note. " +
		"Сначала выполни нужные вызовы инструментов через tool calling API, затем дай краткий итог. " +
		"Не пиши вызовы инструментов текстом и не используй XML вроде <tool_call>. " +
		"Если инструмент вернул status error, сообщи об ошибке честно."
)

// Message is one chat message for the Ollama API.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
}

// ToolCall is a function call requested by the model.
type ToolCall struct {
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and arguments from the model.
type ToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// WithToolSystem prepends the tool-enabled system prompt.
func WithToolSystem(messages []Message) []Message {
	out := make([]Message, 0, len(messages)+1)
	out = append(out, Message{Role: RoleSystem, Content: ToolSystemPrompt})
	out = append(out, messages...)
	return out
}
