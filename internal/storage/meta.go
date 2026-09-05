package storage

import (
	"fmt"
	"strings"
	"time"
)

type noteFields struct {
	Created   time.Time
	Modified  time.Time
	Section   string
	Important bool
}

func parseFrontmatter(raw string) (noteFields, string, bool) {
	raw = strings.TrimPrefix(raw, "\uFEFF")
	if !strings.HasPrefix(raw, "---") {
		return noteFields{}, raw, false
	}

	rest := raw[3:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}

	end := strings.Index(rest, "\n---")
	if end < 0 {
		return noteFields{}, raw, false
	}

	block := rest[:end]
	body := rest[end+4:]
	if strings.HasPrefix(body, "\r\n") {
		body = body[2:]
	} else if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}

	fields := noteFields{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "created":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				fields.Created = t
			}
		case "modified":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				fields.Modified = t
			}
		case "section":
			fields.Section = strings.TrimSpace(val)
		case "important":
			fields.Important = strings.EqualFold(val, "true") || val == "1"
		}
	}

	if fields.Created.IsZero() && !fields.Modified.IsZero() {
		fields.Created = fields.Modified
	}
	if fields.Modified.IsZero() && !fields.Created.IsZero() {
		fields.Modified = fields.Created
	}
	if fields.Created.IsZero() {
		return noteFields{}, raw, false
	}
	return fields, body, true
}

func formatFrontmatter(f noteFields) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("created: %s\n", f.Created.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("modified: %s\n", f.Modified.UTC().Format(time.RFC3339)))
	if f.Section != "" {
		b.WriteString(fmt.Sprintf("section: %s\n", f.Section))
	}
	if f.Important {
		b.WriteString("important: true\n")
	}
	b.WriteString("---\n")
	return b.String()
}

func noteBody(raw string) string {
	_, body, ok := parseFrontmatter(raw)
	if ok {
		return body
	}
	return raw
}

func fieldsFromStat(mod time.Time) noteFields {
	if mod.IsZero() {
		mod = time.Now().UTC()
	}
	return noteFields{Created: mod, Modified: mod}
}

func noteFieldsFromRaw(raw string, mod time.Time) noteFields {
	if fields, _, ok := parseFrontmatter(raw); ok {
		return fields
	}
	return fieldsFromStat(mod)
}
