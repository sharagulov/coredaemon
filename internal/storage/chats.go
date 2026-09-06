package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	chatsFileName    = ".chats.json"
	maxStoredChats   = 20
	maxChatTitleLen  = 80
	maxChatMessages  = 200
)

var ErrInvalidChats = errors.New("invalid chats")

// Chat is one conversation in the notes assistant.
type Chat struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Scope    string            `json:"scope,omitempty"`
	Messages []json.RawMessage `json:"messages"`
}

// ChatStore is the persisted chat list and the active conversation.
type ChatStore struct {
	Chats    []Chat `json:"chats"`
	ActiveID string `json:"activeId"`
}

func (n *Notes) chatsPath() string {
	return filepath.Join(n.dir, chatsFileName)
}

// LoadChats reads notes/.chats.json. Missing file is an empty store.
func (n *Notes) LoadChats() (ChatStore, error) {
	data, err := os.ReadFile(n.chatsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return ChatStore{Chats: []Chat{}}, nil
		}
		return ChatStore{}, fmt.Errorf("read chats: %w", err)
	}
	var store ChatStore
	if err := json.Unmarshal(data, &store); err != nil {
		return ChatStore{}, fmt.Errorf("parse chats: %w", err)
	}
	if store.Chats == nil {
		store.Chats = []Chat{}
	}
	return store, nil
}

// SaveChats writes notes/.chats.json.
func (n *Notes) SaveChats(store ChatStore) error {
	cleaned, err := sanitizeChatStore(store)
	if err != nil {
		return err
	}
	for i := range cleaned.Chats {
		cleaned.Chats[i].Scope = n.sanitizeChatScope(cleaned.Chats[i].Scope)
	}
	data, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return fmt.Errorf("encode chats: %w", err)
	}
	data = append(data, '\n')
	if err := writeFileAtomic(n.chatsPath(), data); err != nil {
		return fmt.Errorf("write chats: %w", err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".chats-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(path)
		if err := os.Rename(tmpName, path); err != nil {
			_ = os.Remove(tmpName)
			return fmt.Errorf("replace: %w", err)
		}
	}
	return nil
}

func sanitizeChatStore(in ChatStore) (ChatStore, error) {
	if len(in.Chats) > maxStoredChats {
		return ChatStore{}, ErrInvalidChats
	}

	out := ChatStore{Chats: make([]Chat, 0, len(in.Chats))}
	seen := make(map[string]struct{}, len(in.Chats))
	for _, c := range in.Chats {
		id := strings.TrimSpace(c.ID)
		if !validChatID(id) {
			return ChatStore{}, ErrInvalidChats
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}

		title := strings.TrimSpace(c.Title)
		if title == "" {
			title = "Новый чат"
		}
		if runes := []rune(title); len(runes) > maxChatTitleLen {
			title = string(runes[:maxChatTitleLen])
		}

		msgs := c.Messages
		if msgs == nil {
			msgs = []json.RawMessage{}
		}
		if len(msgs) > maxChatMessages {
			msgs = append([]json.RawMessage{}, msgs[len(msgs)-maxChatMessages:]...)
		}

		out.Chats = append(out.Chats, Chat{ID: id, Title: title, Scope: strings.TrimSpace(c.Scope), Messages: msgs})
	}

	active := strings.TrimSpace(in.ActiveID)
	if _, ok := seen[active]; ok {
		out.ActiveID = active
	} else if len(out.Chats) > 0 {
		out.ActiveID = out.Chats[0].ID
	}
	return out, nil
}

func validChatID(id string) bool {
	if id == "" || len(id) > 80 {
		return false
	}
	if strings.ContainsAny(id, `/\:`) || strings.Contains(id, "..") {
		return false
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
