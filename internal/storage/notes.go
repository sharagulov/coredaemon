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
	"time"
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
	Name       string `json:"name"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
	Section    string `json:"section,omitempty"`
	Important  bool   `json:"important,omitempty"`
}

// NoteSummary is used in the notes list.
type NoteSummary struct {
	Name       string `json:"name"`
	Title      string `json:"title"`
	Preview    string `json:"preview"`
	CreatedAt  string `json:"created_at,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
	Section    string `json:"section,omitempty"`
	Important  bool   `json:"important,omitempty"`
}

// NoteMetaInput patches note frontmatter without changing body.
type NoteMetaInput struct {
	Section   *string
	Important *bool
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
	if err := n.migrateLegacyTrash(); err != nil {
		_ = n.Close()
		return nil, err
	}
	if err := n.purgeExpiredTrash(); err != nil {
		_ = n.Close()
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
		st, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("stat note %q: %w", rel, err)
		}
		meta := noteFieldsFromRaw(string(data), st.ModTime())
		body := noteBody(string(data))
		out = append(out, NoteSummary{
			Name:       rel,
			Title:      displayTitle(rel, body),
			Preview:    preview(body),
			CreatedAt:  meta.Created.UTC().Format(time.RFC3339),
			ModifiedAt: meta.Modified.UTC().Format(time.RFC3339),
			Section:    meta.Section,
			Important:  meta.Important,
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
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat note: %w", err)
	}
	meta := noteFieldsFromRaw(string(data), st.ModTime())
	body := noteBody(string(data))
	return &Note{
		Name:       rel,
		Content:    body,
		CreatedAt:  meta.Created.UTC().Format(time.RFC3339),
		ModifiedAt: meta.Modified.UTC().Format(time.RFC3339),
		Section:    meta.Section,
		Important:  meta.Important,
	}, nil
}

// Save creates or overwrites a note file.
func (n *Notes) Save(name, content string) (*Note, error) {
	return n.saveNote(name, content, nil)
}

// SaveWithMeta creates or overwrites a note and optionally sets section/important.
func (n *Notes) SaveWithMeta(name, content string, patch *NoteMetaInput) (*Note, error) {
	return n.saveNote(name, content, patch)
}

func (n *Notes) saveNote(name, content string, meta *NoteMetaInput) (*Note, error) {
	path, err := n.filePath(name)
	if err != nil {
		return nil, err
	}
	content = noteBody(content)
	if len(content) > MaxNoteSize {
		return nil, ErrContentTooLarge
	}

	now := time.Now().UTC()
	fields := noteFields{Created: now, Modified: now}
	if data, err := os.ReadFile(path); err == nil {
		st, statErr := os.Stat(path)
		if statErr != nil {
			return nil, fmt.Errorf("stat note: %w", statErr)
		}
		fields = noteFieldsFromRaw(string(data), st.ModTime())
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read note: %w", err)
	}
	fields.Modified = now
	if meta != nil {
		if meta.Section != nil {
			if err := n.validateSectionID(*meta.Section); err != nil {
				return nil, err
			}
			fields.Section = *meta.Section
		}
		if meta.Important != nil {
			fields.Important = *meta.Important
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create note dir: %w", err)
	}
	stored := formatFrontmatter(fields) + content
	if err := os.WriteFile(path, []byte(stored), 0o644); err != nil {
		return nil, fmt.Errorf("write note: %w", err)
	}
	rel, err := normalizeRelPath(name)
	if err != nil {
		return nil, err
	}
	if err := n.upsertIndex(rel, content); err != nil {
		return nil, err
	}
	return noteFromFields(rel, fields, content), nil
}

// Rename changes the note filename from a display title, keeping the folder.
func (n *Notes) Rename(oldName, title string) (*Note, error) {
	oldRel, err := normalizeRelPath(oldName)
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Заметка"
	}

	dir := path.Dir(oldRel)
	if dir == "." {
		dir = ""
	}
	newRel, err := n.uniqueRel(dir, slugFromTitle(title), oldRel)
	if err != nil {
		return nil, err
	}
	if newRel == oldRel {
		return n.Get(oldRel)
	}

	oldPath, err := n.filePath(oldRel)
	if err != nil {
		return nil, err
	}
	newPath, err := n.filePath(newRel)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(oldPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("stat note: %w", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, fmt.Errorf("rename note: %w", err)
	}
	if err := n.removeIndex(oldRel); err != nil {
		return nil, err
	}
	note, err := n.Get(newRel)
	if err != nil {
		return nil, err
	}
	if err := n.upsertIndex(newRel, note.Content); err != nil {
		return nil, err
	}
	return note, nil
}

func (n *Notes) uniqueRel(dir, slug, keep string) (string, error) {
	if slug == "" {
		slug = "note"
	}
	for i := 0; i < 10_000; i++ {
		base := slug + ".md"
		if i > 0 {
			base = fmt.Sprintf("%s-%d.md", slug, i+1)
		}
		cand := base
		if dir != "" {
			cand = dir + "/" + base
		}
		if err := validateName(cand); err != nil {
			return "", err
		}
		if cand == keep {
			return cand, nil
		}
		abs, err := n.filePath(cand)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return cand, nil
			}
			return "", fmt.Errorf("stat note: %w", err)
		}
	}
	return "", fmt.Errorf("could not allocate unique note name")
}

// UpdateMeta changes section/important flags without editing body.
func (n *Notes) UpdateMeta(name string, meta NoteMetaInput) (*Note, error) {
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

	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat note: %w", err)
	}
	fields := noteFieldsFromRaw(string(data), st.ModTime())
	body := noteBody(string(data))
	fields.Modified = time.Now().UTC()

	if meta.Section != nil {
		if err := n.validateSectionID(*meta.Section); err != nil {
			return nil, err
		}
		fields.Section = *meta.Section
	}
	if meta.Important != nil {
		fields.Important = *meta.Important
	}

	stored := formatFrontmatter(fields) + body
	if err := os.WriteFile(path, []byte(stored), 0o644); err != nil {
		return nil, fmt.Errorf("write note: %w", err)
	}
	rel, err := normalizeRelPath(name)
	if err != nil {
		return nil, err
	}
	return noteFromFields(rel, fields, body), nil
}

func (n *Notes) validateSectionID(id string) error {
	if id == "" {
		return nil
	}
	if _, reserved := reservedSectionIDs[id]; reserved {
		return ErrSectionInvalid
	}
	sections, err := n.ListSections()
	if err != nil {
		return err
	}
	for _, s := range sections {
		if s.ID == id {
			return nil
		}
	}
	return ErrSectionNotFound
}

func noteFromFields(rel string, fields noteFields, body string) *Note {
	return &Note{
		Name:       rel,
		Content:    body,
		CreatedAt:  fields.Created.UTC().Format(time.RFC3339),
		ModifiedAt: fields.Modified.UTC().Format(time.RFC3339),
		Section:    fields.Section,
		Important:  fields.Important,
	}
}

func displayTitle(name, content string) string {
	content = noteBody(content)
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
	content = noteBody(content)
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
