package ai

// Tool is an Ollama function tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes one callable function.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// NoteTools returns tool schemas for note file operations.
func NoteTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "create_note",
				Description: "Create a new .md note file on disk",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"title", "content"},
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "Human title with normal capitalization, e.g. Кактусы. Not a filename or slug.",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Full note body",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "append_to_note",
				Description: "Append text to an existing .md note file",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"filename", "content"},
					"properties": map[string]any{
						"filename": map[string]any{
							"type":        "string",
							"description": "Existing note file name, e.g. tasks.md",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Text to append",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_note",
				Description: "Read the contents of an existing .md note file",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"filename"},
					"properties": map[string]any{
						"filename": map[string]any{
							"type":        "string",
							"description": "Note file name, e.g. tasks.md",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "trash_note",
				Description: "Move a note to the trash. The file is not permanently deleted and can be restored from notes/.trash.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"filename"},
					"properties": map[string]any{
						"filename": map[string]any{
							"type":        "string",
							"description": "Note path, e.g. tasks.md or automotive-brand/REUS.md",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "search_notes",
				Description: "Search all notes by keywords. Use before answering questions about existing notes.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"query"},
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search words, e.g. рецепт пирога or четверг",
						},
					},
				},
			},
		},
	}
}
