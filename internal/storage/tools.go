package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	multiHyphen          = regexp.MustCompile(`-{2,}`)
)

// BlockedMutationMsg is returned when the model tries to delete or move notes.
const BlockedMutationMsg = "Удаление и перемещение недоступны"

// ToolResult is returned to the model after a tool call.
type ToolResult struct {
	Status   string      `json:"status"`
	File     string      `json:"file,omitempty"`
	Title    string      `json:"title,omitempty"`
	Content  string      `json:"content,omitempty"`
	Hits     []SearchHit `json:"hits,omitempty"`
	Found    int         `json:"found,omitempty"`
	Query    string      `json:"query,omitempty"`
	Error    string      `json:"error,omitempty"`
	Previous string      `json:"-"`
	NewFile  bool        `json:"-"`
}

// CreateNote creates a new note file from a title and content.
func (n *Notes) CreateNote(title, content string) (*Note, error) {
	title = cleanTitle(title)
	name, err := n.uniqueName(slugFromTitle(title))
	if err != nil {
		return nil, err
	}
	return n.Save(name, stripTitleHeading(title, content))
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

// RevertChanges undoes agent writes: restore previous bodies, then trash created files.
func (n *Notes) RevertChanges(created []string, previous map[string]string) error {
	trash := make([]string, 0, len(created))
	seen := make(map[string]struct{}, len(created))
	for _, name := range created {
		rel, err := normalizeRelPath(normalizeFilename(name))
		if err != nil {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		trash = append(trash, rel)
	}

	for name, content := range previous {
		rel, err := normalizeRelPath(normalizeFilename(name))
		if err != nil {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		if _, err := n.Save(rel, content); err != nil {
			return err
		}
	}

	for _, rel := range trash {
		if _, err := n.Trash(rel); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

// RunTool executes one of the allowed note tools.
func (n *Notes) RunTool(toolName string, args json.RawMessage) (ToolResult, error) {
	return n.runTool(toolName, args, "")
}

// RunToolScoped executes a note tool limited to notes in scope.
func (n *Notes) RunToolScoped(toolName string, args json.RawMessage, scope string) (ToolResult, error) {
	return n.runTool(toolName, args, strings.TrimSpace(scope))
}

func (n *Notes) runTool(toolName string, args json.RawMessage, scope string) (ToolResult, error) {
	switch toolName {
	case "create_note":
		var p struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return ToolResult{Status: "error", Error: "invalid arguments"}, nil
		}
		if err := rejectForeignScript(p.Title, p.Content); err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		title := cleanTitle(p.Title)
		note, err := n.CreateNote(title, p.Content)
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		note, err = n.applyScopeToNote(note.Name, scope)
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
		if err := rejectForeignScript(p.Content); err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		name := normalizeFilename(p.Filename)
		existing, err := n.Get(name)
		isNew := errors.Is(err, ErrNotFound)
		if err != nil && !isNew {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		if !isNew && !NoteMatchesScope(existing.Section, existing.Important, scope) {
			return ToolResult{Status: "error", Error: "note not in scope"}, nil
		}
		var previous string
		if !isNew {
			previous = existing.Content
		}
		note, err := n.AppendToNote(p.Filename, p.Content)
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		if isNew {
			note, err = n.applyScopeToNote(note.Name, scope)
			if err != nil {
				return ToolResult{Status: "error", Error: err.Error()}, nil
			}
		}
		return ToolResult{Status: "success", File: note.Name, Previous: previous, NewFile: isNew}, nil

	case "read_note":
		var p struct {
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return ToolResult{Status: "error", Error: "invalid arguments"}, nil
		}
		note, err := n.Get(normalizeFilename(p.Filename))
		if err != nil {
			if hit, ok := n.lookupNoteHit(p.Filename); ok {
				note, err = n.Get(hit.File)
			}
		}
		if errors.Is(err, ErrNotFound) {
			return ToolResult{Status: "error", Error: "note not found"}, nil
		}
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		if !NoteMatchesScope(note.Section, note.Important, scope) {
			return ToolResult{Status: "error", Error: "note not in scope"}, nil
		}
		return ToolResult{Status: "success", File: note.Name, Content: note.Content}, nil

	case "search_notes":
		var p struct {
			Query string `json:"query"`
		}
		if len(bytes.TrimSpace(args)) > 0 {
			if err := json.Unmarshal(args, &p); err != nil {
				return ToolResult{Status: "error", Error: "invalid arguments"}, nil
			}
		}
		query := stripNameWrap(p.Query)
		hits, err := n.Search(query)
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		hits, err = n.filterHitsByScope(hits, scope)
		if err != nil {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
		if hits == nil {
			hits = []SearchHit{}
		}
		return ToolResult{Status: "success", Hits: hits, Found: len(hits), Query: query}, nil

	case "trash_note", "delete_note", "remove_note", "move_note":
		return ToolResult{Status: "error", Error: BlockedMutationMsg}, nil

	default:
		return ToolResult{Status: "error", Error: "unknown tool"}, nil
	}
}

func (n *Notes) filterHitsByScope(hits []SearchHit, scope string) ([]SearchHit, error) {
	if scope == "" {
		return hits, nil
	}
	list, err := n.List()
	if err != nil {
		return nil, err
	}
	meta := make(map[string]NoteSummary, len(list))
	for _, note := range list {
		meta[note.Name] = note
	}
	out := make([]SearchHit, 0, len(hits))
	for _, hit := range hits {
		note, ok := meta[hit.File]
		if ok && NoteMatchesScope(note.Section, note.Important, scope) {
			out = append(out, hit)
		}
	}
	return out, nil
}

func (n *Notes) applyScopeToNote(name, scope string) (*Note, error) {
	if scope == "" {
		return n.Get(name)
	}
	if scope == "important" {
		empty := ""
		t := true
		return n.UpdateMeta(name, NoteMetaInput{Section: &empty, Important: &t})
	}
	f := false
	return n.UpdateMeta(name, NoteMetaInput{Section: &scope, Important: &f})
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

func stripTitleHeading(title, content string) string {
	body := strings.TrimLeft(content, "\n\r")
	if !sameNoteTitle(headingTitle(body), title) {
		return body
	}
	for {
		line, rest, found := strings.Cut(body, "\n")
		if strings.TrimSpace(line) == "" {
			if !found {
				return ""
			}
			body = rest
			continue
		}
		if found {
			return strings.TrimLeft(rest, "\n\r")
		}
		return ""
	}
}

func sameNoteTitle(a, b string) bool {
	return titleKey(a) != "" && titleKey(a) == titleKey(b)
}

func titleKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			r = unicode.ToLower(r)
			if r == 'ё' {
				r = 'е'
			}
			b.WriteRune(r)
			continue
		}
		b.WriteRune(' ')
	}
	return strings.Join(strings.Fields(b.String()), " ")
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

func rejectForeignScript(parts ...string) error {
	for _, s := range parts {
		for _, r := range s {
			if unicode.Is(unicode.Han, r) {
				return errors.New("текст заметки должен быть на русском")
			}
		}
	}
	return nil
}

func stripNameWrap(name string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(name), "«»\"'`"))
}

func normalizeFilename(name string) string {
	name = stripNameWrap(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	return name
}

func swapLookalikes(s string) string {
	var b strings.Builder
	for _, r := range s {
		if c, ok := latinToCyr[r]; ok {
			b.WriteRune(c)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// foldLookalikes maps Latin/Cyrillic lookalikes only in mixed-script tokens
// (e.g. "Nоски.md" → "Носки.md"). Pure English queries stay untouched.
func foldLookalikes(s string) string {
	hasCyr := false
	hasLatinLookalike := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			hasCyr = true
		default:
			if _, ok := latinToCyr[r]; ok {
				hasLatinLookalike = true
			}
		}
	}
	if hasCyr && hasLatinLookalike {
		return swapLookalikes(s)
	}
	return s
}

var latinToCyr = map[rune]rune{
	'A': 'А', 'B': 'В', 'C': 'С', 'E': 'Е', 'H': 'Н', 'K': 'К', 'M': 'М',
	'N': 'Н', 'O': 'О', 'P': 'Р', 'T': 'Т', 'X': 'Х',
	'a': 'а', 'c': 'с', 'e': 'е', 'o': 'о', 'p': 'р', 'x': 'х', 'y': 'у',
}

func noteHit(note *Note) SearchHit {
	return SearchHit{
		File:    note.Name,
		Title:   displayTitle(note.Name, note.Content),
		Snippet: preview(note.Content),
	}
}

func (n *Notes) lookupNoteHit(query string) (SearchHit, bool) {
	name := normalizeFilename(query)
	if note, err := n.Get(name); err == nil {
		return noteHit(note), true
	}
	if alt := swapLookalikes(name); alt != name {
		if note, err := n.Get(alt); err == nil {
			return noteHit(note), true
		}
	}
	key := titleKey(strings.TrimSuffix(stripNameWrap(query), ".md"))
	if key == "" {
		return SearchHit{}, false
	}
	list, err := n.List()
	if err != nil {
		return SearchHit{}, false
	}
	for _, note := range list {
		base := strings.TrimSuffix(path.Base(note.Name), path.Ext(note.Name))
		if titleKey(base) == key || titleKey(note.Title) == key {
			return SearchHit{File: note.Name, Title: note.Title, Snippet: note.Preview}, true
		}
	}
	return SearchHit{}, false
}
