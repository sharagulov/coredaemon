package storage

import "strings"

// NoteMatchesScope reports whether a note belongs to the chat scope.
// Empty scope means all notes.
func NoteMatchesScope(section string, important bool, scope string) bool {
	if scope == "" {
		return true
	}
	if scope == "important" {
		return important
	}
	return !important && section == scope
}

func (n *Notes) sanitizeChatScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "important" {
		return scope
	}
	if err := n.validateSectionID(scope); err != nil {
		return ""
	}
	return scope
}
