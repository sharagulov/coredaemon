package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	_ "modernc.org/sqlite"
)

const (
	indexFileName  = ".index.db"
	maxSearchQuery = 200
	searchHitLimit = 8
	snippetTokens  = 15
)

// SearchHit is one FTS match returned to the model.
type SearchHit struct {
	File    string `json:"file"`
	Title   string `json:"title,omitempty"`
	Snippet string `json:"snippet"`
}

func (n *Notes) openIndex() error {
	dsn := filepath.Join(n.dir, indexFileName)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return fmt.Errorf("index pragma: %w", err)
	}
	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
			filepath,
			content,
			tokenize = 'unicode61'
		)
	`); err != nil {
		db.Close()
		return fmt.Errorf("create fts table: %w", err)
	}

	n.db = db
	return n.rebuildIndex()
}

// Close releases the search index. Safe on a nil or already-closed Notes.
func (n *Notes) Close() error {
	if n == nil || n.db == nil {
		return nil
	}
	err := n.db.Close()
	n.db = nil
	return err
}

func (n *Notes) rebuildIndex() error {
	tx, err := n.db.Begin()
	if err != nil {
		return fmt.Errorf("begin index rebuild: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM notes_fts`); err != nil {
		return fmt.Errorf("clear index: %w", err)
	}

	ins, err := tx.Prepare(`INSERT INTO notes_fts(filepath, content) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare index insert: %w", err)
	}
	defer ins.Close()

	err = n.walkNotes(func(rel, abs string) error {
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("read note %q for index: %w", rel, err)
		}
		if len(data) > MaxNoteSize {
			return nil
		}
		body := noteBody(string(data))
		if _, err := ins.Exec(rel, body); err != nil {
			return fmt.Errorf("index note %q: %w", rel, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit index rebuild: %w", err)
	}
	return nil
}

func (n *Notes) upsertIndex(name, content string) error {
	if n.db == nil {
		return nil
	}
	if err := n.removeIndex(name); err != nil {
		return err
	}
	if _, err := n.db.Exec(`INSERT INTO notes_fts(filepath, content) VALUES (?, ?)`, name, content); err != nil {
		return fmt.Errorf("index insert %q: %w", name, err)
	}
	return nil
}

func (n *Notes) removeIndex(name string) error {
	if n.db == nil {
		return nil
	}
	if _, err := n.db.Exec(`DELETE FROM notes_fts WHERE filepath = ?`, name); err != nil {
		return fmt.Errorf("index delete %q: %w", name, err)
	}
	return nil
}

// Search runs an FTS5 query over indexed notes.
func (n *Notes) Search(query string) ([]SearchHit, error) {
	match := ftsQuery(query)
	if match == "" {
		return []SearchHit{}, nil
	}

	rows, err := n.db.Query(`
		SELECT filepath, snippet(notes_fts, 1, '<b>', '</b>', '...', ?)
		FROM notes_fts
		WHERE notes_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, snippetTokens, match, searchHitLimit)
	if err != nil {
		return nil, fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()

	hits := make([]SearchHit, 0)
	for rows.Next() {
		var hit SearchHit
		if err := rows.Scan(&hit.File, &hit.Snippet); err != nil {
			return nil, fmt.Errorf("scan search hit: %w", err)
		}
		hit.Title = titleFromName(hit.File)
		if note, err := n.Get(hit.File); err == nil {
			hit.Title = displayTitle(note.Name, note.Content)
		}
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

func ftsQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	if len([]rune(q)) > maxSearchQuery {
		q = string([]rune(q)[:maxSearchQuery])
	}

	var parts []string
	for _, word := range strings.Fields(q) {
		var b strings.Builder
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			parts = append(parts, b.String())
		}
	}
	return strings.Join(parts, " ")
}
