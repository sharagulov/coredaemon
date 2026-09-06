package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/core-daemon/core-daemon/internal/storage"
)

func (a *Agent) loadAttachedNotes(scope string, names []string) (string, bool) {
	names = NormalizeAttachments(names)
	if len(names) == 0 {
		return "", false
	}
	var b strings.Builder
	b.WriteString("Текст прикреплённых заметок с диска:\n")
	loaded := false
	for _, name := range names {
		args, err := json.Marshal(map[string]string{"filename": name})
		if err != nil {
			continue
		}
		res, err := a.notes.RunToolScoped("read_note", args, scope)
		if err != nil || res.Status != "success" {
			continue
		}
		loaded = true
		b.WriteString("\n### ")
		b.WriteString(res.File)
		b.WriteString("\n\n")
		body := strings.TrimSpace(res.Content)
		if body == "" {
			b.WriteString("Заметка пустая.")
		} else {
			b.WriteString(body)
		}
		b.WriteString("\n")
	}
	if !loaded {
		return "", false
	}
	return b.String(), true
}

func (a *Agent) vaultTotal(scope string) int {
	list, err := a.notes.List()
	if err != nil {
		return 0
	}
	n := 0
	for _, note := range list {
		if storage.NoteMatchesScope(note.Section, note.Important, scope) {
			n++
		}
	}
	return n
}

func (a *Agent) vaultCount(scope string) string {
	return fmt.Sprintf("На диске заметок: %d.", a.vaultTotal(scope))
}

func isVaultCountQuery(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	hasCount := strings.Contains(s, "сколько") || strings.Contains(s, "количество")
	return hasCount && strings.Contains(s, "замет")
}

func writeFact(created, updated []string) string {
	var lines []string
	if files := uniqueFiles(created); len(files) > 0 {
		lines = append(lines, "Создано: "+strings.Join(files, ", "))
	}
	if files := uniqueFiles(updated); len(files) > 0 {
		lines = append(lines, "Дополнено: "+strings.Join(files, ", "))
	}
	return strings.Join(lines, "\n")
}

func uniqueFiles(files []string) []string {
	out := make([]string, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

// searchFact reports found files when the model left no usable text of its own.
func searchFact(matches []storage.SearchHit) string {
	files := make([]string, 0, len(matches))
	for _, h := range matches {
		if h.File != "" {
			files = append(files, h.File)
		}
	}
	if len(files) == 0 {
		return ""
	}
	return "Найдено: " + strings.Join(files, ", ")
}

func lastUserText(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser {
			return msgs[i].Content
		}
	}
	return ""
}

func (a *Agent) loadSearchContext(scope, query string) (string, []storage.SearchHit, bool) {
	if !storage.Searchable(query) {
		return "", nil, false
	}
	args, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return "", nil, false
	}
	res, err := a.notes.RunToolScoped("search_notes", args, scope)
	if err != nil || res.Status != "success" {
		return "", nil, false
	}
	hits := res.Hits
	if hits == nil {
		hits = []storage.SearchHit{}
	}
	if len(hits) == 0 {
		return "", hits, false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Поиск по сообщению пользователя, found: %d:\n", len(hits))
	for _, h := range hits {
		b.WriteString("• ")
		b.WriteString(h.File)
		if snip := stripSnippetTags(h.Snippet); snip != "" {
			b.WriteString(" — ")
			b.WriteString(snip)
		}
		b.WriteString("\n")
	}
	return b.String(), hits, true
}

func stripSnippetTags(s string) string {
	s = strings.ReplaceAll(s, "<b>", "")
	s = strings.ReplaceAll(s, "</b>", "")
	return strings.TrimSpace(s)
}
