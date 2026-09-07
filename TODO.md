# Доработки CoreDaemon — пошаговая инструкция

Документ для модели-исполнителя. Пункты независимы, делать строго по одному, в порядке 2 → 3 → 4 (пункт 1 **завершён**).
Пункт 5 без отдельного разрешения пользователя не делать.

## Жёсткие правила работы

1. **Не запускать `go run ./cmd/daemon` и не убивать процессы на `:8080`.** Демон поднимает пользователь.
2. **Не коммитить и не пушить.** Только правки файлов.
3. Минимальный diff под задачу: никаких абстракций «на будущее», хелперов на две строки, переименований заодно.
4. Не создавать новые `.md`-доки, не править README.
5. Порядок внутри пункта всегда такой: **сначала тест, убедиться что он падает, потом код, потом тест зелёный.** Если новый тест проходит до правки кода — тест написан неверно, останавливаться и разбираться.
6. После каждого пункта: `go test ./...` и `go vet ./...` должны быть зелёными целиком, не только в изменённом пакете.
7. `gofmt -l .` в этом репозитории выводит ~9 файлов из-за CRLF-переводов строк. Это было до вас, **не исправлять**. Проверять только то, что ваш файл не добавился в список.
8. Отвечать пользователю по-русски, коротко, по факту.
9. Удаление, перемещение и корзина — только через UI. Не добавлять AI-инструменты для этого ни при каких условиях.

## Как проверять руками

Go-изменения и веб-ассеты вшиты в бинарник через `embed`, поэтому после правок нужен перезапуск демона.
Запускать его самостоятельно нельзя — написать пользователю: «перезапусти демона и проверь такой-то сценарий»,
и указать точный сценарий из раздела «Проверка руками» соответствующего пункта.

---

# Пункт 1. `append_to_note` тихо создаёт дубликат вместо дописывания — **завершено**

## Проблема

Пользователь просит «допиши в заметку про мышек». Модель зовёт `append_to_note` с именем файла,
которое угадала неточно — например `Мышки.md` вместо реального `Мышь-полевая.md`. Файл не находится,
и вместо дописывания создаётся **вторая заметка**. Отчёт при этом честный («Создано»), поэтому баг
незаметен, пока не откроешь дерево заметок и не увидишь дубль.

## Доказательство, что это недосмотр, а не решение

В том же файле у `read_note` есть резолв неточного имени через `n.lookupNoteHit(...)`, а у
`append_to_note` его нет. Хелпер уже написан и уже консервативен: он пробует точное имя, потом
свёртку латинских похожих букв (`Nоски.md` → `Носки.md`), потом сравнение по `titleKey` с именами
и заголовками из `n.List()`. Полнотекстового поиска внутри нет, поэтому случайное совпадение
исключено — использовать его для записи безопасно.

## Файл

`internal/storage/tools.go`, ветка `case "append_to_note":` внутри `func (n *Notes) runTool`.

## Шаг 1.1 — тест

В `internal/storage/tools_test.go` добавить два теста (проверить, что в импортах есть `strings`):

```go
func TestRunTool_appendResolvesExistingName(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("Мышь-полевая.md", "первая строка"); err != nil {
		t.Fatal(err)
	}

	res, err := n.RunTool("append_to_note", []byte(`{"filename":"мышь полевая","content":"вторая строка"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || res.File != "Мышь-полевая.md" || res.NewFile {
		t.Fatalf("res = %+v", res)
	}

	list, err := n.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	note, err := n.Get("Мышь-полевая.md")
	if err != nil || !strings.Contains(note.Content, "первая строка") || !strings.Contains(note.Content, "вторая строка") {
		t.Fatalf("note = %+v, err = %v", note, err)
	}
}

func TestRunTool_appendStillCreatesUnknownNote(t *testing.T) {
	n := openTest(t)
	res, err := n.RunTool("append_to_note", []byte(`{"filename":"Совсем-новая.md","content":"текст"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || !res.NewFile || res.File != "Совсем-новая.md" {
		t.Fatalf("res = %+v", res)
	}
}
```

Запустить `go test ./internal/storage/ -run TestRunTool_append`.
Ожидаемо: первый тест падает (создался второй файл, `list` = 2), второй проходит.
Если первый тест проходит сразу — значит правка уже сделана, дальше не идти.

## Шаг 1.2 — код

Найти в ветке `case "append_to_note":` этот фрагмент:

```go
		name := normalizeFilename(p.Filename)
		existing, err := n.Get(name)
		isNew := errors.Is(err, ErrNotFound)
		if err != nil && !isNew {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
```

Заменить на:

```go
		name := normalizeFilename(p.Filename)
		existing, err := n.Get(name)
		isNew := errors.Is(err, ErrNotFound)
		if isNew {
			// Неточное имя от модели не должно плодить дубликат: сначала ищем существующую заметку.
			if hit, ok := n.lookupNoteHit(p.Filename); ok {
				name = hit.File
				existing, err = n.Get(name)
				isNew = errors.Is(err, ErrNotFound)
			}
		}
		if err != nil && !isNew {
			return ToolResult{Status: "error", Error: err.Error()}, nil
		}
```

Ниже в той же ветке найти вызов записи:

```go
		note, err := n.AppendToNote(p.Filename, p.Content)
```

Заменить аргумент на уже разрешённое имя:

```go
		note, err := n.AppendToNote(name, p.Content)
```

Больше в этой ветке ничего не менять. Поведение «имени вообще нет в хранилище → создаём новую заметку»
сохраняется специально: так модель может дописывать в ещё не существующую заметку, и это рабочий сценарий.

## Шаг 1.3 — проверка

- `go test ./internal/storage/ -run TestRunTool_append` — оба теста зелёные.
- `go test ./...` и `go vet ./...` — зелёные.

## Проверка руками (просить пользователя)

1. Создать в чате заметку: «создай заметку про полевую мышь».
2. Следующим сообщением: «допиши в заметку про полевую мышь, что она рыжая».
3. В дереве заметок должна остаться **одна** заметка, внутри — обе части текста.
   Системный отчёт второго хода должен быть «Дополнено: …», а не «Создано: …».

## Признак, что стало лучше

До правки шаг 2 давал вторую заметку и отчёт «Создано». После правки — «Дополнено» и один файл.
Если отчёт всё ещё «Создано», значит `lookupNoteHit` не нашёл совпадения: попросить у пользователя
точное имя файла и текст запроса, и разбираться отдельно, не расширяя резолв «на всякий случай».

## Чего не делать

- Не подключать сюда `n.Search(...)` или FTS: для записи это слишком широкое совпадение,
  так можно дописать в чужую заметку.
- Не превращать «файл не найден» в ошибку инструмента — сломается создание через append.

---

# Пункт 2. Индекс поиска не видит правки файлов извне

## Проблема

Заметку отредактировали не через UI, а во внешнем редакторе (Obsidian, блокнот). Поиск продолжает
находить старый текст и не находит новый, пока не перезапустить демона.

## Доказательство

Синхронизация индекса сравнивает только количество файлов:

```go
	if indexed == len(list) {
		return nil
	}
	return n.rebuildIndex()
```

Правка содержимого количество не меняет, поэтому `rebuildIndex` не вызывается.
Использовать `ModifiedAt` из `n.List()` для этой проверки нельзя: `noteFieldsFromRaw` берёт дату
из frontmatter заметки, если он есть, а внешний редактор frontmatter не обновляет. Значит смотреть
надо на файловую систему — `os.Stat`.

## Файлы

- `internal/storage/notes.go` — структура `Notes`.
- `internal/storage/index.go` — `syncIndex`, `rebuildIndex`, `upsertIndex`, `removeIndex`.

## Шаг 2.1 — тест

В `internal/storage/index_test.go` добавить (в импортах нужны `os`, `path/filepath`, `time`):

```go
func TestSearch_seesExternalEdit(t *testing.T) {
	n := openTest(t)
	if _, err := n.Save("dog.md", "Шарик — собака"); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Search("шарик"); err != nil {
		t.Fatal(err)
	}

	// правка мимо Save — так пишет Obsidian
	file := filepath.Join(n.dir, "dog.md")
	if err := os.WriteFile(file, []byte("Мурзик — кот\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(file, future, future); err != nil {
		t.Fatal(err)
	}

	hits, err := n.Search("мурзик")
	if err != nil || len(hits) != 1 || hits[0].File != "dog.md" {
		t.Fatalf("new text not indexed: %+v, err = %v", hits, err)
	}
	stale, err := n.Search("шарик")
	if err != nil || len(stale) != 0 {
		t.Fatalf("stale text still indexed: %+v, err = %v", stale, err)
	}
}
```

Запустить `go test ./internal/storage/ -run TestSearch_seesExternalEdit`.
Ожидаемо: падает на первом же `Fatalf` — «мурзик» не найден.

## Шаг 2.2 — код

В `internal/storage/notes.go` добавить поле в структуру:

```go
// Notes reads and writes .md files in dir and keeps an FTS5 search index.
type Notes struct {
	dir   string
	db    *sql.DB
	stamp string
}
```

В `internal/storage/index.go` добавить два метода (рядом с `syncIndex`):

```go
// indexStamp fingerprints the notes dir by file count and newest mtime. Frontmatter can carry
// a stale "modified" field, so the stamp reads the filesystem instead of note metadata.
func (n *Notes) indexStamp() (string, error) {
	count := 0
	var newest int64
	err := n.walkNotes(func(rel, abs string) error {
		st, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("stat note %q: %w", rel, err)
		}
		count++
		if mod := st.ModTime().UnixNano(); mod > newest {
			newest = mod
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", count, newest), nil
}

func (n *Notes) markIndexed() {
	if stamp, err := n.indexStamp(); err == nil {
		n.stamp = stamp
	}
}
```

Заменить целиком `func (n *Notes) syncIndex() error` на:

```go
func (n *Notes) syncIndex() error {
	if n.db == nil {
		return nil
	}
	stamp, err := n.indexStamp()
	if err != nil {
		return err
	}
	if stamp == n.stamp {
		return nil
	}
	return n.rebuildIndex()
}
```

Добавить `n.markIndexed()` ровно в три места, каждое — перед финальным `return nil` соответствующей функции:

- в `rebuildIndex` — после успешного `tx.Commit()`;
- в `upsertIndex` — после успешного `INSERT`;
- в `removeIndex` — после успешного `DELETE`.

Пример для `removeIndex`:

```go
func (n *Notes) removeIndex(name string) error {
	if n.db == nil {
		return nil
	}
	if _, err := n.db.Exec(`DELETE FROM notes_fts WHERE filepath = ?`, name); err != nil {
		return fmt.Errorf("index delete %q: %w", name, err)
	}
	n.markIndexed()
	return nil
}
```

Смысл: правки через UI обновляют индекс точечно и сразу освежают отпечаток, поэтому лишних
перестроений не будет. Любая правка мимо приложения меняет mtime, отпечаток расходится, индекс
перестраивается на следующем поиске.

## Шаг 2.3 — проверка

- `go test ./internal/storage/ -run TestSearch_seesExternalEdit` — зелёный.
- `go test ./...` и `go vet ./...` — зелёные. Особое внимание на `TestSearch_*` и `TestFtsQuery`:
  они не должны сломаться.

## Проверка руками (просить пользователя)

1. Открыть любую заметку и запомнить редкое слово из неё.
2. Отредактировать этот файл в `notes/` внешним редактором: убрать это слово, добавить другое редкое.
3. В чате спросить про новое слово — заметка должна найтись, бейдж появиться.
4. Спросить про старое слово — заметка находиться не должна.

## Признак, что стало лучше

До правки шаг 3 давал «Ничего не найдено» до перезапуска демона. После — находит сразу.

## Чего не делать

- Не добавлять файловый watcher, горутины и `fsnotify`. Отпечаток считается только при поиске
  и при записи через приложение — этого достаточно.
- Не менять схему FTS-таблицы и токенизатор.

Проверено: все записи через приложение идут через `saveNote` → `upsertIndex`, переименование —
через `removeIndex` + `upsertIndex`, корзина — через `removeIndex`. Поэтому после добавления
`markIndexed()` в эти три функции обычная работа в UI лишних перестроений индекса не вызывает.

---

# Пункт 3. Нет бюджета контекста: вложение может вытеснить системный промпт

## Проблема

`MaxNoteSize` — 1 MiB, а `num_ctx` у модели — 8192 токена. Текст прикреплённой заметки уходит
в промпт целиком, туда же уходит результат `read_note`. Достаточно одной большой заметки, чтобы
Ollama начала обрезать историю: правила поведения выпадают из окна, и агент «внезапно тупеет»
без единой ошибки в логе.

## Файлы

- `internal/ai/context.go` — `loadAttachedNotes`.
- `internal/ai/agent.go` — `encodeToolResult`.

## Шаг 3.1 — тест

В `internal/ai/context_test.go` добавить (в импортах нужны `bytes`, `strings`):

```go
func TestAgent_clipsHugeAttachment(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"ок"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })
	if _, err := notes.Save("Большая.md", strings.Repeat("текст ", 20000)); err != nil {
		t.Fatal(err)
	}

	agent := NewAgent(New(srv.URL, "m"), notes)
	if _, err := agent.Chat(context.Background(), []Message{{
		Role:    RoleUser,
		Content: "перескажи",
	}}, nil, "", "Большая.md"); err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(seen, []byte("текст обрезан")) {
		t.Fatal("clip marker missing")
	}
	if len([]rune(string(seen))) > 30000 {
		t.Fatalf("prompt too big: %d runes", len([]rune(string(seen))))
	}
}
```

Запустить `go test ./internal/ai/ -run TestAgent_clipsHugeAttachment`.
Ожидаемо: падает — маркера нет, промпт под 120 000 рун.

## Шаг 3.2 — код

В `internal/ai/context.go` добавить константу и хелпер (рядом с `loadAttachedNotes`):

```go
// maxNoteChars caps one note injected into the prompt. num_ctx is 8192 tokens, so a 1 MiB
// note would push the system prompt out of the window and the rules would stop working.
const maxNoteChars = 6000

func clipNote(body string) string {
	runes := []rune(body)
	if len(runes) <= maxNoteChars {
		return body
	}
	return string(runes[:maxNoteChars]) + "\n… текст обрезан."
}
```

В `loadAttachedNotes` найти:

```go
		body := strings.TrimSpace(res.Content)
		if body == "" {
			b.WriteString("Заметка пустая.")
		} else {
			b.WriteString(body)
		}
```

Заменить на:

```go
		body := strings.TrimSpace(res.Content)
		if body == "" {
			b.WriteString("Заметка пустая.")
		} else {
			b.WriteString(clipNote(body))
		}
```

В `internal/ai/agent.go`, в `encodeToolResult`, перед финальным `json.Marshal(result)` добавить
обрезку результата чтения (`result` — копия, менять её безопасно):

```go
	if name == "read_note" && result.Status == "success" {
		result.Content = clipNote(result.Content)
	}
	body, _ := json.Marshal(result)
	return body
```

## Шаг 3.3 — проверка

- `go test ./internal/ai/ -run TestAgent_clipsHugeAttachment` — зелёный.
- `go test ./...` и `go vet ./...` — зелёные. Тест `TestAgent_injectsAttachedNote` должен остаться
  зелёным: короткие заметки обрезаться не должны.

## Проверка руками (просить пользователя)

1. Прикрепить в чате большую заметку (несколько тысяч слов) и спросить «о чём тут».
2. Ответ должен быть по существу и на русском, без срыва в англоязычный или бессвязный текст.
3. Следом в том же чате попросить «создай заметку про кошек» — инструмент должен вызваться,
   то есть правила из системного промпта не потерялись после большого вложения.

## Признак, что стало лучше

До правки после большого вложения агент часто переставал звать инструменты и отвечал текстом
(системный промпт вытеснялся). После правки поведение на большом и на маленьком вложении одинаковое.

## Чего не делать

- Не менять `MaxNoteSize`: это лимит хранилища, он ни при чём.
- Не поднимать `num_ctx` в `chatOptions`: удвоение KV-кэша может выкинуть модель на CPU,
  и станет медленнее в разы. Это отдельное решение пользователя, не побочный эффект правки.
- Не резать `search_notes`-сниппеты, они и так короткие.

---

# Пункт 4. Не проверяется выдуманная запись

## Проблема

Симметрия к уже закрытой дыре с удалением. Модель может ответить «Создал заметку Кошки.md»,
не вызвав `create_note`. На диске ничего нет, `notesChanged == false`, но текст уходит пользователю
как обычный ответ, и пользователь считает, что заметка есть.

Для удаления это уже закрыто в `agent.go` через `claimsMutation`. Для записи — нет.

## Файлы

- `internal/ai/prompts.go` — новая константа.
- `internal/ai/toolcall.go` — регулярка и проверка.
- `internal/ai/agent.go` — вызов проверки.

## Шаг 4.1 — тест

В `internal/ai/tools_test.go`:

```go
func TestAgent_writeClaimWithoutToolIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Создал заметку Кошки.md"}}`))
	}))
	defer srv.Close()

	notes, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notes.Close() })

	agent := NewAgent(New(srv.URL, "m"), notes)
	result, err := agent.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "создай заметку про кошек"},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != NoWriteMsg || !result.System || result.NotesChanged {
		t.Fatalf("result = %+v", result)
	}
	list, err := notes.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
}
```

В `internal/ai/toolcall_test.go`:

```go
func TestClaimsWrite(t *testing.T) {
	for _, s := range []string{"Создал заметку Кошки.md", "Заметка создана", "Дописал в заметку факты"} {
		if !claimsWrite(s) {
			t.Fatalf("missed claim: %q", s)
		}
	}
	for _, s := range []string{"Создание заметок доступно в интерфейсе", "Что добавить в заметку?", "Ты добавил соль в тесто"} {
		if claimsWrite(s) {
			t.Fatalf("false claim: %q", s)
		}
	}
}
```

Запустить `go test ./internal/ai/ -run "TestAgent_writeClaim|TestClaimsWrite"`.
Ожидаемо: не компилируется (`NoWriteMsg` и `claimsWrite` ещё не существуют) — это нормальный
«красный» шаг, идти дальше.

## Шаг 4.2 — код

В `internal/ai/prompts.go`, внутрь того же блока `const`, рядом с `EmptySearchMsg`:

```go
	// NoWriteMsg replaces a reply that claims a write the tools never performed.
	NoWriteMsg = "Ничего не записано на диск, повтори запрос"
```

В `internal/ai/toolcall.go`, в блок `var` с регулярками, рядом с `doneMutation`:

```go
	// doneWrite matches a finished write ("Создал заметку Кошки.md"). Nouns and infinitives stay
	// out of it, so "создание заметок" and "что добавить" are not treated as reports.
	doneWrite = regexp.MustCompile(`(?i)создал|записал|дописал|добавил|(?:создан|записан|дополнен)[аоы]?(?:$|[^\p{L}])`)
```

Рядом с `claimsMutation` добавить функцию по тому же образцу:

```go
// claimsWrite reports whether the reply announces a note write. The note mention keeps retold
// note bodies ("ты добавил соль в тесто") out of the check.
func claimsWrite(s string) bool {
	if !doneWrite.MatchString(s) {
		return false
	}
	low := strings.ToLower(s)
	return strings.Contains(low, ".md") || strings.Contains(low, "заметк")
}
```

В `internal/ai/agent.go` найти уже существующую проверку и добавить вторую сразу под ней:

```go
			if claimsMutation(content) {
				return finish(storage.BlockedMutationMsg, true), nil
			}
			if claimsWrite(content) {
				return finish(NoWriteMsg, true), nil
			}
```

Дополнительное условие `!notesChanged` здесь **не нужно и добавлять его нельзя**: в начале этой ветки
уже стоит `if notesChanged { return finish(writeFact(...), true), nil }`, то есть до этих строк
исполнение доходит только когда на диске ничего не изменилось.

## Шаг 4.3 — проверка

- `go test ./internal/ai/ -run "TestAgent_writeClaim|TestClaimsWrite"` — зелёные.
- `go test ./...` и `go vet ./...` — зелёные. Особенно важно, что остались зелёными
  `TestAgent_toolLoop`, `TestAgent_createsEveryNoteAcrossTurns`, `TestAgent_createsEveryNoteInOneTurn`:
  при реальном создании заметок текст модели вообще не используется, и новая проверка туда попадать
  не должна.

## Проверка руками (просить пользователя)

1. Попросить «создай заметку про кошек» — должна появиться заметка и системный отчёт «Создано: …»
   с бейджем. Новая проверка не должна ломать нормальный сценарий.
2. Попросить что-нибудь пересказать из заметки, где есть слова «добавил»/«создал» — пересказ должен
   доходить как обычный ответ, а не подменяться на «Ничего не записано».

## Признак, что стало лучше

Фраза «создал заметку» без реальной записи больше не может появиться в чате.
Если пункт 2 из проверки руками сломался — регулярка слишком широкая: сузить её, а не отключать проверку.

## Чего не делать

- Не делать повторный запрос к модели («keyword-retry») при ложном заявлении: это запрещено
  правилами проекта. Только детерминированная подмена текста.
- Не расширять регулярку до инфинитивов (`создать`, `добавить`) — это планы, а не отчёты,
  на них проверка срабатывать не должна.

---

# Пункт 5. Планировщик со структурированным выводом (НЕ ДЕЛАТЬ без разрешения)

Это не багфикс, а изменение формы агента. Делать только если пользователь прямо попросит.

Идея: на запросы «создай N заметок» перестать надеяться, что модель N раз честно прокрутит
`create_note` внутри восьми ходов `maxToolTurns`. Вместо этого одним запросом получить от модели
JSON-список заголовков и текстов по схеме (Ollama умеет `format` со JSON-схемой), провалидировать
его в Go и самим выполнить N вызовов `create_note`. Тогда пять заметок — всегда ровно пять.

Почему это отдельная задача: появляется второй режим работы агента и второй путь ошибок,
нужно решить, как определять «это запрос на пакетное создание» без запрещённых эвристик по
ключевым словам, и что делать при частичном успехе. Прежде чем писать код — согласовать с
пользователем сам подход.

---

# Финальный чеклист перед сдачей

- [ ] Пункты сделаны по одному, каждый со своим тестом, который до правки падал.
- [ ] `go test ./...` — зелёный.
- [ ] `go vet ./...` — без замечаний.
- [ ] `gofmt -l .` не содержит файлов, которых там не было до работы.
- [ ] Ничего не закоммичено, демон не запускался.
- [ ] Пользователю отправлено: что изменилось, какие сценарии проверить руками после перезапуска демона.
