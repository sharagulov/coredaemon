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
	indexFileName    = ".index.db"
	maxSearchQuery   = 200
	searchHitLimit   = 8
	uiSearchHitLimit = 50
	snippetTokens    = 15
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
	if _, err := db.Exec(`DROP TABLE IF EXISTS notes_fts`); err != nil {
		db.Close()
		return fmt.Errorf("drop fts table: %w", err)
	}
	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE notes_fts USING fts5(
			filepath UNINDEXED,
			title,
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

	ins, err := tx.Prepare(`INSERT INTO notes_fts(filepath, title, content) VALUES (?, ?, ?)`)
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
		title := indexTitle(rel, body)
		if _, err := ins.Exec(rel, title, body); err != nil {
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
	if _, err := n.db.Exec(`INSERT INTO notes_fts(filepath, title, content) VALUES (?, ?, ?)`, name, indexTitle(name, content), content); err != nil {
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

// Search runs an FTS5 query over indexed notes (AI tool limit).
func (n *Notes) Search(query string) ([]SearchHit, error) {
	return n.search(query, searchHitLimit)
}

// SearchUI runs an FTS5 query with a higher limit for the web UI.
func (n *Notes) SearchUI(query string) ([]SearchHit, error) {
	return n.search(query, uiSearchHitLimit)
}

func (n *Notes) search(query string, limit int) ([]SearchHit, error) {
	terms := ftsTerms(query)
	if len(terms) == 0 {
		return []SearchHit{}, nil
	}
	hits, err := n.searchMatch(strings.Join(terms, " "), limit)
	if err != nil || len(hits) > 0 || len(terms) == 1 {
		return hits, err
	}
	return n.searchMatch(strings.Join(terms, " OR "), limit)
}

func (n *Notes) searchMatch(match string, limit int) ([]SearchHit, error) {
	rows, err := n.db.Query(`
		SELECT filepath, snippet(notes_fts, 2, '<b>', '</b>', '...', ?)
		FROM notes_fts
		WHERE notes_fts MATCH ?
		ORDER BY bm25(notes_fts, 10.0, 1.0)
		LIMIT ?
	`, snippetTokens, match, limit)
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
		if strings.TrimSpace(hit.Snippet) == "" {
			hit.Snippet = hit.Title
		}
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

var searchStop = map[string]bool{
	"найди": true, "найти": true, "поищи": true, "поиск": true, "search": true,
	"заметки": true, "заметку": true, "заметка": true, "заметке": true, "заметках": true,
	"про": true, "для": true, "при": true, "или": true, "что": true, "как": true,
	"это": true, "the": true, "for": true, "мне": true, "меня": true,
	"упоминания": true, "упоминание": true, "упоминаний": true,
}

var ruSuffixes = []string{
	"ами", "ями", "ыми", "ими", "ого", "его", "ему", "ому",
	"ах", "ях", "ом", "ем", "ой", "ей", "ий", "ый", "ое", "ее",
	"ая", "яя", "ие", "ые", "ам", "ям", "ов", "ев", "ью", "ия", "ии",
	"а", "я", "у", "ю", "о", "е", "ы", "и",
}

func ftsQuery(q string) string {
	return strings.Join(ftsTerms(q), " ")
}

func ftsTerms(q string) []string {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	if len([]rune(q)) > maxSearchQuery {
		q = string([]rune(q)[:maxSearchQuery])
	}

	var parts []string
	seen := map[string]struct{}{}
	for _, word := range strings.Fields(q) {
		var b strings.Builder
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				b.WriteRune(unicode.ToLower(r))
			}
		}
		tok := b.String()
		if tok == "" || searchStop[tok] || (len([]rune(tok)) <= 2 && !hasDigit(tok)) {
			continue
		}
		term := stemToken(tok) + "*"
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		parts = append(parts, term)
	}
	return parts
}

func hasDigit(s string) bool {
	for _, r := range s {
		if unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func stemToken(s string) string {
	rs := []rune(s)
	for _, suf := range ruSuffixes {
		sr := []rune(suf)
		if len(rs)-len(sr) < 4 {
			continue
		}
		if strings.HasSuffix(s, suf) {
			return string(rs[:len(rs)-len(sr)])
		}
	}
	return s
}

func indexTitle(rel, body string) string {
	name := titleFromName(rel)
	if h := headingTitle(body); h != "" && !strings.EqualFold(h, name) {
		return name + " " + h
	}
	return name
}
