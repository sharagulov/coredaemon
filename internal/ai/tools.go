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
				Description: "Create one new .md note file on disk. Call once per note.",
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
							"description": "Note body in Russian. Do not repeat the title as a markdown heading.",
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
				Name:        "update_note",
				Description: "Replace the entire body of an existing .md note. Do not create a new file. Pass the full new body, not a fragment.",
				Parameters: map[string]any{
					"type":     "object",
					"required": []string{"filename", "content"},
					"properties": map[string]any{
						"filename": map[string]any{
							"type":        "string",
							"description": "Existing note file name, e.g. Чай.md",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Full new note body in Russian.",
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
				Name:        "search_notes",
				Description: "Search notes by keywords via FTS. Requires a query; empty query returns no hits.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search words, e.g. рецепт пирога or мерседес.",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}
}
