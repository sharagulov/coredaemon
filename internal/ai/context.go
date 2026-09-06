package ai

import (
	"encoding/json"
	"strings"
	"unicode"
)

func lastUserText(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			return messages[i].Content
		}
	}
	return ""
}

func contextFiles(messages []Message) []string {
	text := lastUserText(messages)
	if text == "" {
		return nil
	}
	head := text
	var names []string
	if i := strings.Index(text, "\n\nКонтекст:"); i >= 0 {
		head = text[:i]
		for _, part := range strings.Split(text[i+len("\n\nКонтекст:"):], ",") {
			if name := strings.TrimSpace(part); name != "" {
				names = append(names, name)
			}
		}
	}
	names = append(names, mdNamesIn(head)...)
	return uniqNames(names)
}

func mdNamesIn(text string) []string {
	var names []string
	var b strings.Builder
	flush := func() {
		name := strings.TrimSpace(b.String())
		b.Reset()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			names = append(names, name)
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '.' || r == '-' || r == '_' || r == '/' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return names
}

func uniqNames(names []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}

func (a *Agent) loadAttachedNotes(scope string, messages []Message) (string, bool) {
	names := contextFiles(messages)
	if len(names) == 0 {
		return "", false
	}
	var b strings.Builder
	b.WriteString("Текст заметок с диска. Отвечай только по нему.\n")
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
