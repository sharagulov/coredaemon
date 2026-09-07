package storage

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	driveIndexName = ".index.db"
	driveScanBatch = 200
	driveScanMax   = 200_000
)

// DriveEntry is one folder or file in the drive index.
type DriveEntry struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	MIME  string `json:"mime"`
	Kind  string `json:"kind"`
	MTime int64  `json:"mtime"`
}

// Drive indexes files under root and serves list/mkdir/upload.
type Drive struct {
	root string
	db   *sql.DB
}

// OpenDrive creates root if needed and opens drive_index.
func OpenDrive(dir string) (*Drive, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve drive dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create drive dir: %w", err)
	}
	d := &Drive{root: abs}
	if err := d.openIndex(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Drive) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	err := d.db.Close()
	d.db = nil
	return err
}

func (d *Drive) openIndex() error {
	db, err := sql.Open("sqlite", filepath.Join(d.root, driveIndexName))
	if err != nil {
		return fmt.Errorf("open drive index: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS drive_index (
			path TEXT PRIMARY KEY,
			parent TEXT NOT NULL,
			name TEXT NOT NULL,
			is_dir INTEGER NOT NULL,
			size INTEGER NOT NULL,
			mime TEXT NOT NULL,
			kind TEXT NOT NULL,
			mtime_ns INTEGER NOT NULL,
			seen INTEGER NOT NULL DEFAULT 1
		);
		CREATE INDEX IF NOT EXISTS drive_index_parent
			ON drive_index(parent, is_dir DESC, name COLLATE NOCASE);
	`); err != nil {
		db.Close()
		return fmt.Errorf("create drive_index: %w", err)
	}
	d.db = db
	return nil
}

func (d *Drive) List(rel string) ([]DriveEntry, error) {
	rel, err := normalizeDrivePath(rel)
	if err != nil {
		return nil, err
	}
	rows, err := d.db.Query(`
		SELECT path, name, is_dir, size, mime, kind, mtime_ns
		FROM drive_index WHERE parent = ?
		ORDER BY is_dir DESC, name COLLATE NOCASE
	`, rel)
	if err != nil {
		return nil, fmt.Errorf("list drive: %w", err)
	}
	defer rows.Close()
	out := make([]DriveEntry, 0)
	for rows.Next() {
		var e DriveEntry
		var isDir int
		if err := rows.Scan(&e.Path, &e.Name, &isDir, &e.Size, &e.MIME, &e.Kind, &e.MTime); err != nil {
			return nil, err
		}
		e.IsDir = isDir == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *Drive) Mkdir(rel string) (*DriveEntry, error) {
	rel, err := normalizeDrivePath(rel)
	if err != nil || rel == "" {
		return nil, ErrInvalidName
	}
	abs, err := d.absPath(rel)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	for p := rel; p != ""; p = driveParent(p) {
		if err := d.upsertStat(p); err != nil {
			return nil, err
		}
	}
	return d.statEntry(rel)
}

func (d *Drive) SaveFile(rel string, r io.Reader) (*DriveEntry, error) {
	rel, err := normalizeDrivePath(rel)
	if err != nil || rel == "" {
		return nil, ErrInvalidName
	}
	abs, err := d.absPath(rel)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		return nil, ErrInvalidName
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	tmp := abs + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return nil, copyErr
		}
		return nil, closeErr
	}
	if err := os.Rename(tmp, abs); err != nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			if remErr := os.Remove(abs); remErr == nil {
				err = os.Rename(tmp, abs)
			}
		}
		if err != nil {
			_ = os.Remove(tmp)
			return nil, err
		}
	}
	for p := driveParent(rel); p != ""; p = driveParent(p) {
		if err := d.upsertStat(p); err != nil {
			return nil, err
		}
	}
	if err := d.upsertStat(rel); err != nil {
		return nil, err
	}
	return d.statEntry(rel)
}

func (d *Drive) OpenFile(rel string) (*os.File, *DriveEntry, error) {
	rel, err := normalizeDrivePath(rel)
	if err != nil || rel == "" {
		return nil, nil, ErrInvalidName
	}
	abs, err := d.absPath(rel)
	if err != nil {
		return nil, nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if st.IsDir() {
		return nil, nil, ErrInvalidName
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	e, err := d.statEntry(rel)
	if err != nil {
		kind, mime := sniffDriveFile(abs, driveBase(rel), false)
		e = &DriveEntry{
			Path:  rel,
			Name:  driveBase(rel),
			Size:  st.Size(),
			MIME:  mime,
			Kind:  kind,
			MTime: st.ModTime().UnixNano(),
		}
	}
	return f, e, nil
}

func (d *Drive) Scan() error {
	if _, err := d.db.Exec(`UPDATE drive_index SET seen = 0`); err != nil {
		return err
	}
	n := 0
	batch := 0
	err := filepath.WalkDir(d.root, func(abs string, de os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if de.IsDir() && abs != d.root && strings.HasPrefix(de.Name(), ".") {
			return filepath.SkipDir
		}
		if !de.IsDir() && (strings.HasPrefix(de.Name(), ".") || strings.HasSuffix(de.Name(), ".part")) {
			return nil
		}
		rel, err := filepath.Rel(d.root, abs)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if _, err := normalizeDrivePath(rel); err != nil {
			if de.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if err := d.upsertStat(rel); err != nil {
			return err
		}
		n++
		if n > driveScanMax {
			return fmt.Errorf("too many drive entries")
		}
		batch++
		if batch >= driveScanBatch {
			batch = 0
			runtime.Gosched()
			time.Sleep(time.Millisecond)
		}
		return nil
	})
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`DELETE FROM drive_index WHERE seen = 0`)
	return err
}

func (d *Drive) StartBackgroundScan() {
	go func() {
		if err := d.Scan(); err != nil {
			log.Printf("drive scan: %v", err)
		}
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := d.Scan(); err != nil {
				log.Printf("drive scan: %v", err)
			}
		}
	}()
}

func (d *Drive) upsertStat(rel string) error {
	abs, err := d.absPath(rel)
	if err != nil {
		return err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	kind, mime := sniffDriveFile(abs, driveBase(rel), st.IsDir())
	isDir := 0
	size := st.Size()
	if st.IsDir() {
		isDir = 1
		size = 0
	}
	_, err = d.db.Exec(`
		INSERT INTO drive_index(path, parent, name, is_dir, size, mime, kind, mtime_ns, seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(path) DO UPDATE SET
			parent=excluded.parent,
			name=excluded.name,
			is_dir=excluded.is_dir,
			size=excluded.size,
			mime=excluded.mime,
			kind=excluded.kind,
			mtime_ns=excluded.mtime_ns,
			seen=1
	`, rel, driveParent(rel), driveBase(rel), isDir, size, mime, kind, st.ModTime().UnixNano())
	return err
}

func (d *Drive) Remove(rel string) error {
	rel, err := normalizeDrivePath(rel)
	if err != nil || rel == "" {
		return ErrInvalidName
	}
	abs, err := d.absPath(rel)
	if err != nil {
		return err
	}
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	if st.IsDir() {
		err = os.RemoveAll(abs)
	} else {
		err = os.Remove(abs)
	}
	if err != nil {
		return err
	}
	_, err = d.db.Exec(
		`DELETE FROM drive_index WHERE path = ? OR path GLOB ?`,
		rel, rel+"/*",
	)
	return err
}

func (d *Drive) statEntry(rel string) (*DriveEntry, error) {
	var e DriveEntry
	var isDir int
	err := d.db.QueryRow(`
		SELECT path, name, is_dir, size, mime, kind, mtime_ns
		FROM drive_index WHERE path = ?
	`, rel).Scan(&e.Path, &e.Name, &isDir, &e.Size, &e.MIME, &e.Kind, &e.MTime)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.IsDir = isDir == 1
	return &e, nil
}
