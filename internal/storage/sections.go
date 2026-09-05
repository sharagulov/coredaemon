package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

var (
	ErrSectionExists   = errors.New("section already exists")
	ErrSectionInvalid  = errors.New("invalid section name")
	ErrSectionNotFound = errors.New("section not found")
)

const sectionsFileName = ".sections.json"

var (
	sectionSlugInvalid = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	sectionMultiHyphen = regexp.MustCompile(`-{2,}`)
	reservedSectionIDs = map[string]struct{}{
		"":          {},
		"all":       {},
		"important": {},
		"trash":     {},
	}
)

// Section is a user-defined note category.
type Section struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type sectionsFile struct {
	Sections []Section `json:"sections"`
}

func (n *Notes) sectionsPath() string {
	return filepath.Join(n.dir, sectionsFileName)
}

// ListSections returns custom sections sorted by name.
func (n *Notes) ListSections() ([]Section, error) {
	data, err := os.ReadFile(n.sectionsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Section{}, nil
		}
		return nil, fmt.Errorf("read sections: %w", err)
	}
	var file sectionsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse sections: %w", err)
	}
	if file.Sections == nil {
		return []Section{}, nil
	}
	out := append([]Section{}, file.Sections...)
	sortSections(out)
	return out, nil
}

// CreateSection adds a custom section.
func (n *Notes) CreateSection(name string) (Section, error) {
	name = cleanSectionName(name)
	if name == "" {
		return Section{}, ErrSectionInvalid
	}
	id := slugFromSectionName(name)
	if _, reserved := reservedSectionIDs[id]; reserved {
		return Section{}, ErrSectionInvalid
	}

	file, err := n.loadSectionsFile()
	if err != nil {
		return Section{}, err
	}
	for _, s := range file.Sections {
		if s.ID == id || strings.EqualFold(s.Name, name) {
			return Section{}, ErrSectionExists
		}
	}

	sec := Section{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	file.Sections = append(file.Sections, sec)
	if err := n.saveSectionsFile(file); err != nil {
		return Section{}, err
	}
	return sec, nil
}

// DeleteSection removes a custom section and clears it from notes.
func (n *Notes) DeleteSection(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrSectionInvalid
	}
	if _, reserved := reservedSectionIDs[id]; reserved {
		return ErrSectionInvalid
	}

	file, err := n.loadSectionsFile()
	if err != nil {
		return err
	}

	found := false
	kept := make([]Section, 0, len(file.Sections))
	for _, s := range file.Sections {
		if s.ID == id {
			found = true
			continue
		}
		kept = append(kept, s)
	}
	if !found {
		return ErrSectionNotFound
	}

	file.Sections = kept
	if err := n.saveSectionsFile(file); err != nil {
		return err
	}
	return n.clearNoteSection(id)
}

func (n *Notes) clearNoteSection(sectionID string) error {
	list, err := n.List()
	if err != nil {
		return err
	}
	empty := ""
	for _, sum := range list {
		if sum.Section != sectionID {
			continue
		}
		if _, err := n.UpdateMeta(sum.Name, NoteMetaInput{Section: &empty}); err != nil {
			return err
		}
	}
	return nil
}

func (n *Notes) loadSectionsFile() (sectionsFile, error) {
	list, err := n.ListSections()
	if err != nil {
		return sectionsFile{}, err
	}
	return sectionsFile{Sections: list}, nil
}

func (n *Notes) saveSectionsFile(file sectionsFile) error {
	sortSections(file.Sections)
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sections: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(n.sectionsPath(), data, 0o644); err != nil {
		return fmt.Errorf("write sections: %w", err)
	}
	return nil
}

func sortSections(list []Section) {
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
}

func cleanSectionName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Join(strings.Fields(name), " ")
	if len([]rune(name)) > 64 {
		name = string([]rune(name)[:64])
	}
	return name
}

func slugFromSectionName(name string) string {
	slug := sectionSlugInvalid.ReplaceAllString(name, "")
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = sectionMultiHyphen.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-.")
	slug = strings.ToLower(slug)
	if slug == "" {
		return "section"
	}
	runes := []rune(slug)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
