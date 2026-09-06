package ai

import "strings"

const maxAttachments = 16

// NormalizeAttachments deduplicates attachment paths from the client.
func NormalizeAttachments(names []string) []string {
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
		if len(out) >= maxAttachments {
			break
		}
	}
	return out
}
