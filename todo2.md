# Диск CoreDaemon — архитектура и пошаговая реализация

Документ для модели-исполнителя. Это не медиа-хаб: универсальное локальное хранилище
(аналог Диска), корень на диске — `data/drive/`. Заметки (`notes/`) не трогать.

Пункты делать строго по одному: D1 → D2 → D3. **D4 (проводник) не делать** — интерфейс придёт отдельными макетами Figma.
Пункт D5 — только проводка `/drive` и `.gitignore`, без UI-поиска.
Пункт D6 (удаление/корзина диска) без отдельного разрешения не делать.

## Жёсткие правила

1. **Не запускать `go run ./cmd/daemon` и не убивать процессы на `:8080`.** Демон поднимает пользователь.
2. **Не коммитить и не пушить.**
3. Минимальный diff. Не тащить S3, minio, ffmpeg, thumbnails-пайплайн, WebDAV, FUSE.
4. Не писать README. Этот файл — единственная инструкция.
5. Сначала тест (красный), потом код, потом тест зелёный.
6. После каждого пункта: `go test ./...` и `go vet ./...`.
7. `gofmt -l .` — не чинить старые CRLF. Свой файл в список не добавлять.
8. Отвечать пользователю по-русски, коротко.
9. AI-tools для диска не добавлять. Удаление файлов диска через модель — запрещено.
10. Слои не смешивать: API не читает `data/drive/` в обход `storage.Drive`, UI не трогает диск.

## Как проверять руками

Ассеты вшиты через `embed`. После правок написать: «перезапусти демона и проверь …»
из раздела «Проверка руками» текущего пункта.

---

# Архитектурные решения

Почему так, а не иначе. Код ниже следует этим решениям, не спорит с ними.

### Хранилище

- **Отдельный корень `data/drive/`, отдельный тип `storage.Drive`.** Заметки остаются `.md` + FTS5. Диск — любые файлы. Смешивать индексы нельзя: у заметок frontmatter и `normalizeRelPath` требует `.md`.
- **Истина — файловая система.** SQLite (`data/drive/.index.db`) — кэш для списка и поиска. Файл есть на диске → он существует. Индекс отстал — следующий list/scan подтянет.
- **`drive_index` с колонкой `parent`, не adjacency через рекурсию и не JSON-дерево.** `GET /api/drive/list?path=foo` = `WHERE parent = ?`. На 100k файлов это индексный lookup, не полный walk на каждый клик.
- **`path` — PRIMARY KEY, slash-separated (`photos/2024/cat.jpg`).** Один канонический вид, как у заметок. Windows-слэши нормализуются на входе.
- **Нет content-hash в v1.** SHA на 100k видео убьёт CPU и диск при старте. Детект изменений: `(size, mtime_ns)`. Хеш — отдельное решение, если появится дедуп.
- **Скрытые сегменты (`.trash`, `.index.db`, `.something`) не индексировать и не отдавать.** Как у заметок.

### Сканер

- **Фоновая горутина, не блокирует `Open` и HTTP.** Старт демона отдаёт UI сразу. Первый list может быть пустым на доли секунды на холодном индексе — допустимо; повторный list уже из БД.
- **Не fsnotify.** На Windows с сетевыми папками и 100k файлов это отдельный класс багов. Скан по запросу + периодический тик (30с) + скан после upload/mkdir.
- **Incremental walk + mark-and-sweep.** `UPDATE seen=0`, walk батчами по 200 `INSERT/UPDATE`, `DELETE WHERE seen=0`. Не держим 100k путей в `map` дольше одного прохода — батч пишет и забывает. `seen` — колонка поколения, не RAM-множество всех путей после walk.
- **Yield каждые 200 файлов:** `runtime.Gosched()` + пауза 1ms. Не вешаем один P на минуту.
- **Лимит 200_000 записей.** Дальше walk останавливается с логом. Это защита RAM/индекса, не продуктовый «безлимит».
- **`http.DetectContentType` только на первые 512 байт и только если расширение неизвестно.** Картинка/видео/pdf почти всегда закрываются таблицей расширений. Целый файл не читаем.

### API

- **Пути через `{path...}` в Go 1.22, не через `?path=` в stream.** List может оставить query (`list?path=`), stream — `GET /api/drive/file/{path...}`: Range и `ServeContent` так проще, чем экранировать слэши в одном сегменте. Имя в ТЗ было `/stream/{path}` — смысл тот же, путь с слэшами в ServeMux корректнее как `{path...}`.
- **Upload: `r.MultipartReader()` + `io.Copy` в файл, не `ParseMultipartForm`.** `ParseMultipartForm(32<<20)` держит до 32 МиБ в RAM на часть. Reader копирует кусками (~32 КиБ) на диск. Лимит одной части — 4 ГиБ (защита от бесконечного потока), не 40 МиБ: видео иначе нельзя.
- **`http.ServeContent`, не ручной Range.** stdlib уже умеет `Range`, `If-Range`, `206`. `ServeFile` тоже можно, но `ServeContent` даёт явный `Content-Type` из индекса.
- **normalizeDrivePath:** без `..`, `.`, пустых сегментов, скрытых имён, `<>:"|?*`, лимит глубины 24, длины 1024. Корневой list — пустая строка (разрешена, в отличие от заметок).
- **mkdir и upload атомарны относительно индекса:** сначала FS, потом upsert. Если upsert упал — файл на диске остаётся, следующий scan подберёт. Обратное (индекс без файла) хуже.

### Фронтенд

- **Карточка «Облако» на домашнем экране уже есть (`data-open` сейчас disabled).** Включаем её, экран `drive` рядом с `notes`, не внутри заметок.
- **Виджеты в `internal/web/ui/drive/`, не разметка в `app.js`.** Как чат: фабрики, баррель `ui/drive/index.js`, embed `ui/drive/*.js`.
- **Превью картинок — `<img src="/api/drive/file/...">`, видео — `<video controls>` с тем же URL.** Отдельный thumbnail-сервис не делаем: браузер сам запросит Range. Для списка папки картинки можно показать маленьким `<img>`, браузер кэширует. На 1000 фото в папке — lazy `loading="lazy"`.
- **Хлебные крошки — единственная навигация вверх.** Клик по папке = `path = item.path`. Не плодить второе дерево в сайдбаре в v1.
- **Поиск по диску в v1 не обязателен.** Колонка `name` + индекс есть, UI-поиск — пункт D5, если останется время внутри D4 не делать.

### Чего нет в v1

- Шаринг по ссылке, права, пользователи.
- Транскодинг, обложки, EXIF.
- Корзина диска (D6, отдельно).
- AI-доступ к файлам диска.
- Синхронизация с чужим облаком.

---

# Схема `drive_index`

Файл БД: `data/drive/.index.db` (скрытый, не показывается в list).

```sql
CREATE TABLE IF NOT EXISTS drive_index (
    path     TEXT    PRIMARY KEY, -- "photos/2024/cat.jpg" или "photos/2024"
    parent   TEXT    NOT NULL,    -- "" для корня, "photos" для "photos/2024"
    name     TEXT    NOT NULL,    -- последний сегмент
    is_dir   INTEGER NOT NULL,    -- 0/1
    size     INTEGER NOT NULL,    -- 0 для папок
    mime     TEXT    NOT NULL,    -- "image/jpeg", "inode/directory", …
    kind     TEXT    NOT NULL,    -- folder|image|video|audio|pdf|archive|text|other
    mtime_ns INTEGER NOT NULL,
    seen     INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS drive_index_parent ON drive_index(parent, is_dir DESC, name COLLATE NOCASE);
```

`kind` дублирует грубый класс для UI, чтобы фронт не парсил MIME. Источник истины для стрима — `mime`.

---

# Пункт D1. Storage: путь, MIME, индекс, list/mkdir

## Файлы

- `internal/storage/drive.go` — тип `Drive`, Open, Close, List, Mkdir, Upsert, RemoveIndex.
- `internal/storage/drive_path.go` — `normalizeDrivePath`, `driveFilePath`.
- `internal/storage/drive_mime.go` — расширение + magic 512 байт.
- `internal/storage/drive_test.go` — тесты.

В `.gitignore` добавить:

```
/data/drive/*
!/data/drive/.gitkeep
```

Создать `data/drive/.gitkeep`.

`cmd/daemon/main.go`: `drive, err := storage.OpenDrive("data/drive")`, `defer drive.Close()`. Пока API нет — только Open, чтобы индекс создался. API подключим в D3.

## Шаг D1.1 — тесты пути и MIME

```go
package storage

import "testing"

func TestNormalizeDrivePath(t *testing.T) {
	ok, err := normalizeDrivePath("photos/2024/cat.jpg")
	if err != nil || ok != "photos/2024/cat.jpg" {
		t.Fatalf("got %q, err=%v", ok, err)
	}
	root, err := normalizeDrivePath("")
	if err != nil || root != "" {
		t.Fatalf("root = %q, err=%v", root, err)
	}
	for _, bad := range []string{"../etc/passwd", "a/../b", "/abs", "foo/./bar", ".hidden", "a/.secret/b", "a\\..\\b"} {
		if _, err := normalizeDrivePath(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestDriveKindFromName(t *testing.T) {
	if kind, mime := driveKind("x.jpg", false); kind != "image" || mime != "image/jpeg" {
		t.Fatalf("jpg = %s %s", kind, mime)
	}
	if kind, mime := driveKind("x.mp4", false); kind != "video" || mime != "video/mp4" {
		t.Fatalf("mp4 = %s %s", kind, mime)
	}
	if kind, _ := driveKind("x.pdf", false); kind != "pdf" {
		t.Fatalf("pdf = %s", kind)
	}
	if kind, mime := driveKind("dir", true); kind != "folder" || mime != "inode/directory" {
		t.Fatalf("dir = %s %s", kind, mime)
	}
}
```

`go test ./internal/storage/ -run "TestNormalizeDrivePath|TestDriveKindFromName"` — не компилируется. Идти дальше.

## Шаг D1.2 — код пути и MIME

`internal/storage/drive_path.go`:

```go
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
```

`internal/storage/drive_mime.go`:

```go
package storage

import (
	"net/http"
	"os"
	"path"
	"strings"
)

func driveKind(name string, isDir bool) (kind, mime string) {
	if isDir {
		return "folder", "inode/directory"
	}
	ext := strings.ToLower(path.Ext(name))
	if k, m, ok := driveExt[ext]; ok {
		return k, m
	}
	return "other", "application/octet-stream"
}

func sniffDriveFile(abs, name string, isDir bool) (kind, mime string) {
	kind, mime = driveKind(name, isDir)
	if isDir || kind != "other" {
		return kind, mime
	}
	f, err := os.Open(abs)
	if err != nil {
		return kind, mime
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return kind, mime
	}
	mime = http.DetectContentType(buf[:n])
	return kindFromMIME(mime), mime
}

func kindFromMIME(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case mime == "application/pdf":
		return "pdf"
	case strings.HasPrefix(mime, "text/"):
		return "text"
	default:
		return "other"
	}
}

var driveExt = map[string][/*kind,mime*/]string{} // см. ниже — обычный map[string]struct{k,m string} или два значения
```

Не использовать массив `[2]string` с кривым комментарием. Реализовать так:

```go
var driveExt = map[string]struct{ kind, mime string }{
	".jpg": {kind: "image", mime: "image/jpeg"}, ".jpeg": {kind: "image", mime: "image/jpeg"},
	".png": {kind: "image", mime: "image/png"}, ".gif": {kind: "image", mime: "image/gif"},
	".webp": {kind: "image", mime: "image/webp"}, ".svg": {kind: "image", mime: "image/svg+xml"},
	".mp4": {kind: "video", mime: "video/mp4"}, ".webm": {kind: "video", mime: "video/webm"},
	".mov": {kind: "video", mime: "video/quicktime"}, ".mkv": {kind: "video", mime: "video/x-matroska"},
	".mp3": {kind: "audio", mime: "audio/mpeg"}, ".wav": {kind: "audio", mime: "audio/wav"},
	".flac": {kind: "audio", mime: "audio/flac"}, ".ogg": {kind: "audio", mime: "audio/ogg"},
	".pdf": {kind: "pdf", mime: "application/pdf"},
	".zip": {kind: "archive", mime: "application/zip"}, ".rar": {kind: "archive", mime: "application/vnd.rar"},
	".7z": {kind: "archive", mime: "application/x-7z-compressed"}, ".tar": {kind: "archive", mime: "application/x-tar"},
	".gz": {kind: "archive", mime: "application/gzip"},
	".txt": {kind: "text", mime: "text/plain"}, ".md": {kind: "text", mime: "text/markdown"},
	".csv": {kind: "text", mime: "text/csv"}, ".json": {kind: "text", mime: "application/json"},
	".doc": {kind: "other", mime: "application/msword"},
	".docx": {kind: "other", mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
}
```

И `driveKind` читает `driveExt[ext]`.

## Шаг D1.3 — тест list/mkdir

```go
func TestDrive_listAndMkdir(t *testing.T) {
	d := openDriveTest(t)
	if _, err := d.Mkdir("photos/2024"); err != nil {
		t.Fatal(err)
	}
	root, err := d.List("")
	if err != nil || len(root) != 1 || root[0].Path != "photos" || !root[0].IsDir {
		t.Fatalf("root = %+v, err=%v", root, err)
	}
	photos, err := d.List("photos")
	if err != nil || len(photos) != 1 || photos[0].Path != "photos/2024" {
		t.Fatalf("photos = %+v, err=%v", photos, err)
	}
	if _, err := d.Mkdir("../x"); err == nil {
		t.Fatal("traversal mkdir")
	}
}

func openDriveTest(t *testing.T) *Drive {
	t.Helper()
	d, err := OpenDrive(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}
```

Запустить `go test ./internal/storage/ -run TestDrive_listAndMkdir` — красный.

## Шаг D1.4 — код Drive

`internal/storage/drive.go` — каркас:

```go
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const driveIndexName = ".index.db"

type DriveEntry struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	MIME  string `json:"mime"`
	Kind  string `json:"kind"`
	MTime int64  `json:"mtime"`
}

type Drive struct {
	root string
	db   *sql.DB
}

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
	// индекс всех предков, не только листа
	for p := rel; p != ""; p = driveParent(p) {
		if err := d.upsertStat(p); err != nil {
			return nil, err
		}
	}
	return d.statEntry(rel)
}
```

`upsertStat` / `statEntry`: `os.Stat` → `sniffDriveFile` → `INSERT OR REPLACE`. Папки size=0.

Проводка в `main`: OpenDrive + Close. Пока без MountDrive.

`go test ./internal/storage/ -run TestDrive` и `go test ./...` — зелёные.

---

# Пункт D2. Сканер и запись файла на диск

## Проблема

List видит только то, что попало в индекс через Mkdir. Файлы, скопированные в `data/drive/` снаружи, невидимы.

## Шаг D2.1 — тест

Положить файл мимо API, вызвать `Scan`, list должен его увидеть. Второй файл удалить с диска — после Scan из индекса пропасть.

```go
func TestDrive_scanSeesExternalWrite(t *testing.T) {
	d := openDriveTest(t)
	nested := filepath.Join(d.root, "docs")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "a.pdf"), []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Scan(); err != nil {
		t.Fatal(err)
	}
	docs, err := d.List("docs")
	if err != nil || len(docs) != 1 || docs[0].Name != "a.pdf" || docs[0].Kind != "pdf" {
		t.Fatalf("docs = %+v, err=%v", docs, err)
	}
	if err := os.Remove(filepath.Join(nested, "a.pdf")); err != nil {
		t.Fatal(err)
	}
	if err := d.Scan(); err != nil {
		t.Fatal(err)
	}
	docs, err = d.List("docs")
	if err != nil || len(docs) != 0 {
		t.Fatalf("stale = %+v, err=%v", docs, err)
	}
}
```

Красный — идти дальше.

## Шаг D2.2 — сканер

```go
const (
	driveScanBatch = 200
	driveScanMax   = 200_000
)

func (d *Drive) Scan() error {
	if _, err := d.db.Exec(`UPDATE drive_index SET seen = 0`); err != nil {
		return err
	}
	n := 0
	batch := 0
	err := filepath.WalkDir(d.root, func(abs string, de os.DirEntry, err error) error {
		if err != nil {
			return nil // битая ветка — пропуск, не валим весь скан
		}
		if de.IsDir() && abs != d.root && strings.HasPrefix(de.Name(), ".") {
			return filepath.SkipDir
		}
		if !de.IsDir() && strings.HasPrefix(de.Name(), ".") {
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
		if _, err := d.db.Exec(`UPDATE drive_index SET seen = 1 WHERE path = ?`, rel); err != nil {
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
		_ = d.Scan()
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			_ = d.Scan()
		}
	}()
}
```

`upsertStat` после успешного write должен ставить `seen=1`, иначе ближайший незавершённый scan сотрёт свежую загрузку. Либо upload всегда вызывает `Scan` точечно через upsert с `seen=1`, а фоновый scan атомарный. Правило: **любая запись через Drive делает upsert с seen=1**. Фоновый Scan — единственный, кто обнуляет seen.

`SaveFile(rel, r io.Reader)` — для upload:

```go
func (d *Drive) SaveFile(rel string, r io.Reader) (*DriveEntry, error) {
	rel, err := normalizeDrivePath(rel)
	if err != nil || rel == "" {
		return nil, ErrInvalidName
	}
	abs, err := d.absPath(rel)
	if err != nil {
		return nil, err
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
		_ = os.Remove(tmp)
		return nil, err
	}
	for p := driveParent(rel); p != ""; p = driveParent(p) {
		_ = d.upsertStat(p)
	}
	if err := d.upsertStat(rel); err != nil {
		return nil, err
	}
	return d.statEntry(rel)
}
```

Тест: `SaveFile("docs/a.txt", strings.NewReader("hi"))` → list docs содержит a.txt.

`main`: после OpenDrive вызвать `drive.StartBackgroundScan()`.

`go test ./...` — зелёный.

---

# Пункт D3. HTTP API

## Файлы

- `internal/api/drive.go`
- `internal/api/drive_test.go`
- `cmd/daemon/main.go` — `api.MountDrive(mux, drive)`

## Маршруты

| Метод | Путь | Смысл |
|---|---|---|
| GET | `/api/drive/list` | `?path=` содержимое папки |
| POST | `/api/drive/mkdir` | JSON `{"path":"photos/2024"}` |
| POST | `/api/drive/upload` | multipart: поле `path` (папка), файлы `file` (можно несколько) |
| GET | `/api/drive/file/{path...}` | стрим + Range |

`GET /api/drive/stream/{path}` из ТЗ = этот file-роут. Не плодить синоним.

## Шаг D3.1 — тесты

- list корня после mkdir через HTTP.
- upload маленького файла, list видит, GET file отдаёт тело и `Accept-Ranges`.
- `list?path=../x` → 400.
- `GET /api/drive/file/../x` → 400.

Красный — дальше.

## Шаг D3.2 — хэндлеры

Защита пути: только `normalizeDrivePath` + `absPath`. Никакого `filepath.Join(root, user)` без нормализации.

Upload:

```go
const maxDrivePart = 4 << 30 // 4 GiB

mr, err := r.MultipartReader()
// поля:
//  path — директория назначения (может быть "")
//  file — одна или несколько частей
// каждую file-часть: dest = destDir + "/" + base(filename), SaveFile(dest, io.LimitReader(part, maxDrivePart+1))
// если прочитали больше maxDrivePart — 413, удалить partial
```

`filename` из multipart прогнать через `path.Base` и `normalizeDrivePath` вместе с dest dir. Имя `../../x` станет `x` только после Base — этого мало: Base (`..`) отбросить как invalid.

Стрим:

```go
abs, err := drive.AbsPath(rel) // экспортировать обёртку или метод File(rel) (abs, mime, err)
http.ServeContent(w, r, name, modTime, f)
```

Не читать файл в `[]byte`.

Ошибки: invalid → 400, not found → 404.

`go test ./internal/api/ -run Drive` и `go test ./...` — зелёные.

---

# Пункт D4. UI проводника — ЖДАТЬ МАКЕТЫ FIGMA

Проводник не придумывать. Карточка «Облако» открывает пустой экран с шапкой «Меню / Облако».
Сетку, крошки, превью, кнопки загрузки — только по макетам.

---

# Пункт D4. UI проводника (после макетов)

## Файлы

- `internal/web/ui/drive/explorer.js` — `createDriveExplorer`
- `internal/web/ui/drive/crumbs.js` — крошки
- `internal/web/ui/drive/row.js` — строка файла/папки
- `internal/web/ui/drive/preview.js` — модалка image/video
- `internal/web/ui/drive/index.js` — баррель
- `internal/web/style.css` — BEM `.drive-*`, тёмная тема как у заметок
- `internal/web/index.html` — экран `data-screen="drive"`, карточка Облако enabled
- `internal/web/embed.go` — `ui/drive/*.js`
- `internal/web/app.js` — открытие экрана, без вёрстки строк

## Поведение

1. Карточка «Облако» на домашнем → `body.dataset.screen = "drive"`, `history` как у notes (`/drive` или hash).
2. Шапка: «Меню» назад, заголовок «Облако».
3. Крошки: `Облако / photos / 2024`. Клик по сегменту — list этого path.
4. Кнопки: «Папка», «Загрузить» (`<input type=file multiple>`).
5. Клик по `kind=folder` — войти. Клик по image/video — preview с `/api/drive/file/{encodeURI(path)}`. Иначе — только иконка + имя + размер, клик качает тот же URL (`a[download]` или `window.open`).
6. Картинки в сетке: `<img loading="lazy" src="/api/drive/file/...">`. Не генерировать превью на сервере.

Навигация path хранится в переменной виджета, не в заметках.

## Чего не делать

- Не копировать бенто-карточки заметок.
- Не подключать чат на экране диска в v1.
- Не писать виртуализацию на 100k в одной папке: если папка огромная — list как есть, `loading=lazy` на img. Виртуализация — отдельное решение.

Проверка руками:

1. Перезапусти демона.
2. Меню → Облако. Пусто, крошка «Облако».
3. Создать папку `photos`, войти, загрузить jpg и pdf.
4. jpg открывается в превью, pdf — иконка, скачивается.
5. Положить файл в `data/drive/` проводником ОС, подождать до 30с или перезайти в папку после тика — файл появился.
6. `GET /api/drive/list?path=../etc` — 400.

---

# Пункт D5. Мелочи проводки (если D1–D4 зелёные)

- `GET /drive` в `web.Mount` как `GET /notes`.
- `.gitignore` + `.gitkeep`.
- Лог при старте: `drive data/drive`.
- Не добавлять поиск, если не попросили.

---

# Пункт D6. Корзина диска (НЕ ДЕЛАТЬ без разрешения)

Отдельная задача: `.trash` внутри `data/drive/`, не смешивать с `notes/.trash`. Restore с коллизиями `-2`. AI не имеет tool.

---

# Порядок файлов (итог)

```
data/drive/.gitkeep
internal/storage/drive.go
internal/storage/drive_path.go
internal/storage/drive_mime.go
internal/storage/drive_test.go
internal/api/drive.go
internal/api/drive_test.go
internal/web/ui/drive/*.js
cmd/daemon/main.go          — OpenDrive, MountDrive, StartBackgroundScan
internal/web/embed.go       — ui/drive/*.js
internal/web/index.html     — экран drive, карточка Облако
internal/web/app.js         — переключение экрана
internal/web/style.css      — .drive-*
.gitignore
```

Заметки, чат, Ollama, OpenAI, `TODO.md` / пункт 6 заметок — не в этом объёме.
