package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const trashDirName = ".trash"

// Trash moves a note into notes/.trash, keeping its relative path.
// The file is not deleted; restore it by moving it back out of .trash.
func (n *Notes) Trash(name string) (string, error) {
	rel, err := normalizeRelPath(name)
	if err != nil {
		return "", err
	}

	src, err := n.filePath(rel)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("stat note: %w", err)
	}

	dest, err := n.uniqueTrashPath(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create trash dir: %w", err)
	}
	if err := os.Rename(src, dest); err != nil {
		return "", fmt.Errorf("move to trash: %w", err)
	}
	n.pruneEmptyParents(rel)
	if err := n.removeIndex(rel); err != nil {
		return "", err
	}
	return rel, nil
}

func (n *Notes) uniqueTrashPath(rel string) (string, error) {
	root, err := filepath.Abs(filepath.Join(n.dir, trashDirName))
	if err != nil {
		return "", fmt.Errorf("resolve trash dir: %w", err)
	}

	try := func(candidate string) (string, bool, error) {
		dest, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(candidate)))
		if err != nil {
			return "", false, err
		}
		relToRoot, err := filepath.Rel(root, dest)
		if err != nil || relToRoot == "." || strings.HasPrefix(relToRoot, "..") {
			return "", false, ErrInvalidName
		}
		_, err = os.Stat(dest)
		if os.IsNotExist(err) {
			return dest, true, nil
		}
		if err != nil {
			return "", false, err
		}
		return "", false, nil
	}

	if dest, ok, err := try(rel); ok || err != nil {
		return dest, err
	}

	stem := strings.TrimSuffix(rel, ".md")
	for i := 2; i < 10_000; i++ {
		dest, ok, err := try(fmt.Sprintf("%s-%d.md", stem, i))
		if ok || err != nil {
			return dest, err
		}
	}
	return "", fmt.Errorf("could not allocate trash name")
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
