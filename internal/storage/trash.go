package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	trashDirName   = ".trash"
	trashNoteFile  = "note.md"
	trashMetaFile  = "meta.json"
	trashRetention = 30 * 24 * time.Hour
)

type trashMeta struct {
	Name      string    `json:"name"`
	TrashedAt time.Time `json:"trashed_at"`
}

// TrashItem is a note sitting in the trash, restorable until ExpiresAt.
type TrashItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	TrashedAt time.Time `json:"trashed_at"`
	ExpiresAt time.Time `json:"expires_at"`
	DaysLeft  int       `json:"days_left"`
}

func (n *Notes) trashRoot() string {
	return filepath.Join(n.dir, trashDirName)
}

// Trash moves a note into the trash. It can be restored for 30 days.
func (n *Notes) Trash(name string) (string, error) {
	rel, err := normalizeRelPath(name)
	if err != nil {
		return "", err
	}

	src, err := n.filePath(rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("read note: %w", err)
	}
	if len(data) > MaxNoteSize {
		return "", ErrContentTooLarge
	}

	id, err := n.writeTrashEntry(rel, data, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if err := os.Remove(src); err != nil {
		_ = os.RemoveAll(n.trashEntryDir(id))
		return "", fmt.Errorf("remove note: %w", err)
	}
	n.pruneEmptyParents(rel)
	if err := n.removeIndex(rel); err != nil {
		return "", err
	}
	return rel, nil
}

// ListTrash returns restorable notes and drops entries older than 30 days.
func (n *Notes) ListTrash() ([]TrashItem, error) {
	if err := n.purgeExpiredTrash(); err != nil {
		return nil, err
	}

	root := n.trashRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []TrashItem{}, nil
		}
		return nil, fmt.Errorf("read trash: %w", err)
	}

	out := make([]TrashItem, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		item, err := n.readTrashItem(e.Name(), false)
		if err != nil {
			continue
		}
		out = append(out, *item)
	}
	sortTrash(out)
	return out, nil
}

// GetTrash reads one trash entry, including content.
func (n *Notes) GetTrash(id string) (*TrashItem, error) {
	if err := n.purgeExpiredTrash(); err != nil {
		return nil, err
	}
	return n.readTrashItem(id, true)
}

// Restore moves a trash entry back into notes. Original path is kept when free.
func (n *Notes) Restore(id string) (*Note, error) {
	item, err := n.GetTrash(id)
	if err != nil {
		return nil, err
	}

	dest, err := n.uniqueLivePath(item.Name)
	if err != nil {
		return nil, err
	}
	note, err := n.Save(dest, item.Content)
	if err != nil {
		return nil, err
	}
	_ = os.RemoveAll(n.trashEntryDir(id))
	return note, nil
}

func (n *Notes) writeTrashEntry(name string, content []byte, at time.Time) (string, error) {
	root := n.trashRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create trash dir: %w", err)
	}

	at = at.UTC()
	base := strconv.FormatInt(at.UnixMilli(), 10)
	body, err := json.Marshal(trashMeta{Name: name, TrashedAt: at})
	if err != nil {
		return "", err
	}

	for i := 0; i < 10_000; i++ {
		id := base
		if i > 0 {
			id = fmt.Sprintf("%s-%d", base, i+1)
		}
		dir := filepath.Join(root, id)
		err := os.Mkdir(dir, 0o755)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create trash entry: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, trashNoteFile), content, 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("write trash note: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, trashMetaFile), body, 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("write trash meta: %w", err)
		}
		return id, nil
	}
	return "", fmt.Errorf("could not allocate trash id")
}

func (n *Notes) readTrashItem(id string, withContent bool) (*TrashItem, error) {
	if !validTrashID(id) {
		return nil, ErrInvalidName
	}

	dir := n.trashEntryDir(id)
	raw, err := os.ReadFile(filepath.Join(dir, trashMetaFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read trash meta: %w", err)
	}

	var meta trashMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("parse trash meta: %w", err)
	}
	if _, err := normalizeRelPath(meta.Name); err != nil {
		return nil, ErrInvalidName
	}

	expires := meta.TrashedAt.Add(trashRetention)
	remaining := time.Until(expires)
	if remaining <= 0 {
		_ = os.RemoveAll(dir)
		return nil, ErrNotFound
	}

	data, err := os.ReadFile(filepath.Join(dir, trashNoteFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read trash note: %w", err)
	}

	item := &TrashItem{
		ID:        id,
		Name:      meta.Name,
		Title:     displayTitle(meta.Name, string(data)),
		TrashedAt: meta.TrashedAt,
		ExpiresAt: expires,
		DaysLeft:  daysLeft(remaining),
	}
	if withContent {
		item.Content = string(data)
	}
	return item, nil
}

func (n *Notes) purgeExpiredTrash() error {
	root := n.trashRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read trash: %w", err)
	}

	cutoff := time.Now().Add(-trashRetention)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name(), trashMetaFile))
		if err != nil {
			continue
		}
		var meta trashMeta
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}
		if meta.TrashedAt.Before(cutoff) {
			if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
				return fmt.Errorf("purge trash %q: %w", e.Name(), err)
			}
		}
	}
	return nil
}

func (n *Notes) trashEntryDir(id string) string {
	return filepath.Join(n.trashRoot(), id)
}

func (n *Notes) migrateLegacyTrash() error {
	root := n.trashRoot()
	var files []string
	err := filepath.WalkDir(root, func(abs string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == trashMetaFile {
			return nil
		}
		if name == trashNoteFile {
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(abs), trashMetaFile)); statErr == nil {
				return nil
			}
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		files = append(files, abs)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("scan trash: %w", err)
	}

	for _, abs := range files {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if err := validateName(name); err != nil {
			name = filepath.Base(abs)
			if err := validateName(name); err != nil {
				name = "note.md"
			}
		}
		info, err := os.Stat(abs)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("read legacy trash: %w", err)
		}
		if _, err := n.writeTrashEntry(name, data, info.ModTime().UTC()); err != nil {
			return err
		}
		if err := os.Remove(abs); err != nil {
			return err
		}
		n.pruneEmptyUntil(root, filepath.Dir(abs))
	}
	return nil
}

func (n *Notes) pruneEmptyUntil(root, dir string) {
	for dir != root {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func (n *Notes) uniqueLivePath(rel string) (string, error) {
	rel, err := normalizeRelPath(rel)
	if err != nil {
		return "", err
	}

	free := func(candidate string) (bool, error) {
		p, err := n.filePath(candidate)
		if err != nil {
			return false, err
		}
		_, err = os.Stat(p)
		if err == nil {
			return false, nil
		}
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	if ok, err := free(rel); ok || err != nil {
		if ok {
			return rel, nil
		}
		return "", err
	}

	stem := strings.TrimSuffix(rel, ".md")
	for i := 2; i < 10_000; i++ {
		candidate := fmt.Sprintf("%s-%d.md", stem, i)
		ok, err := free(candidate)
		if ok {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate restore name")
}

func (n *Notes) pruneEmptyParents(rel string) {
	dir := filepath.Dir(filepath.Join(n.dir, filepath.FromSlash(rel)))
	for {
		if dir == n.dir {
			return
		}
		relToRoot, err := filepath.Rel(n.dir, dir)
		if err != nil || relToRoot == "." || strings.HasPrefix(relToRoot, "..") {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func validTrashID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func sortTrash(items []TrashItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].TrashedAt.After(items[j].TrashedAt)
	})
}

func daysLeft(remaining time.Duration) int {
	if remaining <= 0 {
		return 0
	}
	d := remaining / (24 * time.Hour)
	if remaining%(24*time.Hour) != 0 {
		d++
	}
	return int(d)
}
