package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxPathDepth = 16
	maxPathLen   = 512
	maxWalkNotes = 10_000
)

// normalizeRelPath returns a slash-separated path inside the notes root.
func normalizeRelPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.Trim(name, "/")
	if name == "" || len(name) > maxPathLen {
		return "", ErrInvalidName
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		return "", ErrInvalidName
	}

	parts := strings.Split(name, "/")
	if len(parts) == 0 || len(parts) > maxPathDepth {
		return "", ErrInvalidName
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", ErrInvalidName
		}
		if strings.HasPrefix(part, ".") {
			return "", ErrInvalidName
		}
		if strings.ContainsAny(part, `<>:"|?*`) || strings.ContainsRune(part, 0) {
			return "", ErrInvalidName
		}
	}
	return name, nil
}

func validateName(name string) error {
	_, err := normalizeRelPath(name)
	return err
}

func (n *Notes) filePath(name string) (string, error) {
	rel, err := normalizeRelPath(name)
	if err != nil {
		return "", err
	}

	joined := filepath.Join(n.dir, filepath.FromSlash(rel))
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve note path: %w", err)
	}

	root := n.dir + string(os.PathSeparator)
	if abs != n.dir && !strings.HasPrefix(abs, root) {
		return "", ErrInvalidName
	}
	return abs, nil
}

func (n *Notes) walkNotes(fn func(rel, abs string) error) error {
	count := 0
	return filepath.WalkDir(n.dir, func(abs string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if abs != n.dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(n.dir, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err := validateName(rel); err != nil {
			return nil
		}

		count++
		if count > maxWalkNotes {
			return fmt.Errorf("too many notes")
		}
		return fn(rel, abs)
	})
}
