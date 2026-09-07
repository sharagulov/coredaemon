package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxDriveDepth = 24
	maxDrivePath  = 1024
)

func normalizeDrivePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "/") {
		return "", ErrInvalidName
	}
	name = strings.Trim(name, "/")
	if name == "" {
		return "", nil
	}
	if len(name) > maxDrivePath {
		return "", ErrInvalidName
	}
	parts := strings.Split(name, "/")
	if len(parts) > maxDriveDepth {
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

func driveParent(rel string) string {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return ""
	}
	return rel[:i]
}

func driveBase(rel string) string {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return rel
	}
	return rel[i+1:]
}

func (d *Drive) absPath(rel string) (string, error) {
	rel, err := normalizeDrivePath(rel)
	if err != nil {
		return "", err
	}
	joined := d.root
	if rel != "" {
		joined = filepath.Join(d.root, filepath.FromSlash(rel))
	}
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve drive path: %w", err)
	}
	root := d.root + string(os.PathSeparator)
	if abs != d.root && !strings.HasPrefix(abs, root) {
		return "", ErrInvalidName
	}
	return abs, nil
}
