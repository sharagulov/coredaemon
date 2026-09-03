package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

var (
	ErrInvalidName     = errors.New("invalid note name")
	ErrNotFound        = errors.New("note not found")
	ErrContentTooLarge = errors.New("content too large")
)

const (
	MaxNoteSize   = 1 << 20 // 1 MiB
	PreviewLength = 120
)

// Note is a note file with name and content.
type Note struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// NoteSummary is used in the notes list.
type NoteSummary struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Preview string `json:"preview"`
}

// Notes reads and writes .md files in dir and keeps an FTS5 search index.
type Notes struct {
	dir string
	db  *sql.DB
}

// Open opens or creates the notes directory and builds the search index.
func Open(dir string) (*Notes, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve notes dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create notes dir: %w", err)
	}

	n := &Notes{dir: abs}
	if err := n.openIndex(); err != nil {
		return nil, err
	}
	return n, nil
}

// List returns all .md notes, including nested folders.
func (n *Notes) List() ([]NoteSummary, error) {
	out := make([]NoteSummary, 0)
	err := n.walkNotes(func(rel, abs string) error {
		data, err := readNoteHead(abs, 4096)
		if err != nil {
			return fmt.Errorf("read note %q: %w", rel, err)
		}
		if len(data) > MaxNoteSize {
			return nil
		}
		out = append(out, NoteSummary{
			Name:    rel,
			Title:   displayTitle(rel, string(data)),
			Preview: preview(string(data)),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get reads one note by file name.
func (n *Notes) Get(name string) (*Note, error) {
	path, err := n.filePath(name)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read note: %w", err)
	}
	if len(data) > MaxNoteSize {
		return nil, ErrContentTooLarge
	}

	rel, err := normalizeRelPath(name)
	if err != nil {
		return nil, err
	}
	return &Note{Name: rel, Content: string(data)}, nil
}

// Save creates or overwrites a note file.
func (n *Notes) Save(name, content string) (*Note, error) {
	path, err := n.filePath(name)
	if err != nil {
		return nil, err
	}
	if len(content) > MaxNoteSize {
		return nil, ErrContentTooLarge
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create note dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write note: %w", err)
	}
	rel, err := normalizeRelPath(name)
	if err != nil {
		return nil, err
	}
	if err := n.upsertIndex(rel, content); err != nil {
		return nil, err
	}
	return &Note{Name: rel, Content: content}, nil
}

func displayTitle(name, content string) string {
	if title := headingTitle(content); title != "" {
		return title
	}
	return titleFromName(name)
}

func headingTitle(content string) string {
	for _, line := range strings.SplitN(content, "\n", 8) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			return ""
		}
		return strings.TrimSpace(strings.TrimLeft(line, "#"))
	}
	return ""
}

func titleFromName(name string) string {
	base := path.Base(filepath.ToSlash(name))
	base = strings.TrimSuffix(base, path.Ext(base))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.Join(strings.Fields(base), " ")
	return capitalizeFirst(base)
}

func capitalizeFirst(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	runes[0] = unicode.ToTitle(runes[0])
	return string(runes)
}

func preview(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	content = strings.Join(strings.Fields(content), " ")
	runes := []rune(content)
	if len(runes) <= PreviewLength {
		return content
	}
	return string(runes[:PreviewLength]) + "…"
}

func readNoteHead(path string, max int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, max)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}
