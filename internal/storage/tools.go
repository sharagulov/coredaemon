package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	multiHyphen          = regexp.MustCompile(`-{2,}`)
)

// BlockedMutationMsg is returned when the model tries to delete or move notes.
const BlockedMutationMsg = "нельзя удалять или перемещать заметки в целях безопасности"

// ToolResult is returned to the model after a tool call.
type ToolResult struct {
	Status  string      `json:"status"`
	File    string      `json:"file,omitempty"`
	Title   string      `json:"title,omitempty"`
	Content string      `json:"content,omitempty"`
	Hits    []SearchHit `json:"hits,omitempty"`
	Found   int         `json:"found,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// CreateNote creates a new note file from a title and content.
func (n *Notes) CreateNote(title, content string) (*Note, error) {
	title = cleanTitle(title)
	name, err := n.uniqueName(slugFromTitle(title))
	if err != nil {
		return nil, err
	}
	return n.Save(name, withHeading(title, content))
}

// AppendToNote appends content to an existing note.
func (n *Notes) AppendToNote(name, content string) (*Note, error) {
	name = normalizeFilename(name)
	if err := validateName(name); err != nil {
		return nil, err
	}

	note, err := n.Get(name)
	if errors.Is(err, ErrNotFound) {
		return n.Save(name, content)
	}
	if err != nil {
		return nil, err
	}

	joined := note.Content
	if joined != "" && !strings.HasSuffix(joined, "\n") {
		joined += "\n"
	}
	joined += content
	return n.Save(name, joined)
}

// RunTool executes one of the allowed note tools.
func (n *Notes) RunTool(toolName string, args json.RawMessage) (ToolResult, error) {
	switch toolName {
	case "create_note":
		var p struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return ToolResult{Status: "error", Error: "invalid arguments"}, nil
		}
		title := cleanTitle(p.Title)
		note, err := n.CreateNote(title, p.Content)
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		return ToolResult{Status: "success", File: note.Name, Title: title}, nil

	case "append_to_note":
		var p struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return ToolResult{Status: "error", Error: "invalid arguments"}, nil
		}
		note, err := n.AppendToNote(p.Filename, p.Content)
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		return ToolResult{Status: "success", File: note.Name}, nil

	case "read_note":
		var p struct {
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return ToolResult{Status: "error", Error: "invalid arguments"}, nil
		}
		note, err := n.Get(normalizeFilename(p.Filename))
		if errors.Is(err, ErrNotFound) {
			return ToolResult{Status: "error", Error: "note not found"}, nil
		}
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		return ToolResult{Status: "success", File: note.Name, Content: note.Content}, nil

	case "search_notes":
		var p struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return ToolResult{Status: "error", Error: "invalid arguments"}, nil
		}
		hits, err := n.Search(p.Query)
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		if hits == nil {
			hits = []SearchHit{}
		}
		return ToolResult{Status: "success", Hits: hits, Found: len(hits)}, nil

	case "trash_note", "delete_note", "remove_note", "move_note":
		return ToolResult{Status: "error", Error: BlockedMutationMsg}, nil

	default:
		return ToolResult{Status: "error", Error: "unknown tool"}, nil
	}
}

func (n *Notes) uniqueName(slug string) (string, error) {
	if slug == "" {
		slug = "note"
	}

	for i := 0; i < 10_000; i++ {
		candidate := slug + ".md"
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d.md", slug, i+1)
		}
		if err := validateName(candidate); err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(n.dir, candidate)); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", fmt.Errorf("stat note: %w", err)
		}
	}
	return "", fmt.Errorf("could not allocate unique note name")
}

func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.TrimSuffix(title, ".md")
	title = strings.TrimSpace(title)
	if strings.HasPrefix(title, "#") {
		title = strings.TrimSpace(strings.TrimPrefix(title, "#"))
	}
	if title == "" {
		return "Новая заметка"
	}
	return capitalizeFirst(title)
}

func withHeading(title, content string) string {
	if headingTitle(content) != "" {
		return content
	}
	body := strings.TrimLeft(content, "\n")
	if strings.TrimSpace(body) == "" {
		return "# " + title + "\n"
	}
	return "# " + title + "\n\n" + body
}

func slugFromTitle(title string) string {
	slug := invalidFilenameChars.ReplaceAllString(strings.TrimSpace(title), "")
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = multiHyphen.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-.")
	if slug == "" {
		return "note"
	}
	return slug
}

func normalizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	return name
}
