import {
  createMenuOption,
  syncMenuOptions,
  createDropdown,
  createAddMenuButton,
  bindSearchInput,
  createInlineForm,
  createNoteCard as createNoteCardComponent,
  createContextMenu,
  createSubmenuRow,
  createContextAction,
  registerPopupDismiss,
  createMarkdownEditor,
  renderMarkdown,
} from "./ui/index.js";

const FILTER_STORAGE_KEY = "notes-filter";

  const state = {
    notes: [],
    trash: [],
    sections: [],
    view: "notes",
    sortBy: "modified",
    filterBy: localStorage.getItem(FILTER_STORAGE_KEY) || "all",
    filterCreating: false,
    searchQuery: "",
    searchHits: null,
    searchSnippets: {},
    active: null,
    activeTrash: null,
    chat: [],
    sending: false,
    pending: false,
  };

  const els = {
    // Notes workspace
    bento: document.querySelector(".notes-bento"),
    toolbarTitle: document.querySelector(".notes-toolbar__title"),
    homeBtn: document.querySelector(".notes-toolbar__home"),
    addBtn: document.querySelector(".notes-toolbar__add"),
    searchInput: document.querySelector(".notes-toolbar__search"),
    trashBtn: document.querySelector(".notes-toolbar__trash"),
    sortBtn: document.querySelector(".notes-toolbar__sort"),
    sortLabel: document.querySelector(".notes-toolbar__sort-label"),
    sortMenu: document.querySelector(".notes-sort__menu"),
    filterBtn: document.querySelector(".notes-toolbar__filter"),
    filterLabel: document.querySelector(".notes-toolbar__filter-label"),
    filterMenu: document.querySelector(".notes-filter__menu"),
    notesCard: document.querySelector("[data-open=\"notes\"]"),
    readerEmpty: document.querySelector(".notes-reader__empty"),
    readerPanel: document.querySelector(".notes-reader__panel"),
    readerFoot: document.querySelector(".notes-reader__foot"),
    readerAction: document.querySelector(".notes-reader__action"),
    ctxRoot: document.querySelector(".notes-ctx"),
    ctxMenu: document.querySelector(".notes-ctx__menu"),
    // Legacy (mummified — preserved for future Cloud section)
    list: document.querySelector(".sidebar__list"),
    label: document.querySelector(".sidebar__label"),
    messages: document.querySelector(".chat__messages"),
    chatForm: document.querySelector(".chat__input"),
    chatInput: document.querySelector(".chat__input textarea"),
    chatSubmit: document.querySelector(".chat__input button"),
  };

  function noteLabel(name) {
    const note = state.notes.find((n) => n.name === name);
    if (note?.title) return note.title;
    const base = name.replace(/^.*\//, "").replace(/\.md$/i, "");
    return base.replace(/[-_]/g, " ").trim() || name;
  }

  function noteFolder(name) {
    const parts = String(name || "").split("/");
    if (parts.length <= 1) return "";
    return parts.slice(0, -1).join("/");
  }

  function cardPreview(text) {
    let s = String(text || "").trim();
    if (!s) return "Пустая заметка";
    if (s.startsWith("---")) {
      const end = s.indexOf("---", 3);
      if (end !== -1) s = s.slice(end + 3).trim();
    }
    s = s.replace(/^#+\s+/gm, "").replace(/\[\[([^\]|]+)(?:\|[^\]]+)?\]\]/g, "$1");
    s = s.replace(/\s+/g, " ").trim();
    return s || "Пустая заметка";
  }

  const SORT_OPTIONS = [
    { id: "modified", label: "дате изменения" },
    { id: "created", label: "дате создания" },
    { id: "title", label: "алфавиту от А до Я" },
    { id: "title-desc", label: "алфавиту от Я до А" },
  ];

  const BUILTIN_FILTERS = [
    { id: "all", label: "Все" },
    { id: "important", label: "Важные" },
  ];

  function sectionMoveTargets() {
    return [
      { id: "important", label: "Важные" },
      ...state.sections.map((s) => ({ id: s.id, label: s.name })),
    ];
  }

  function noteMatchesSection(note, sectionId) {
    return sectionId === "important" ? !!note?.important : note?.section === sectionId;
  }

  function appendFilterOption(menu, { id, label, deletable = false }) {
    const btn = createMenuOption({
      className: "notes-filter__option",
      id,
      label,
      active: state.filterBy === id,
      dataKey: "filter",
    });
    if (deletable) {
      btn.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        e.stopPropagation();
        openSectionContextMenu(e, id);
      });
    }
    menu.appendChild(btn);
  }

  let sortDropdown;
  let filterDropdown;
  let contextMenu;
  let searchControl;

  function closeSortMenu() {
    sortDropdown?.close();
  }

  function closeContextMenu() {
    contextMenu?.close();
  }

  function dismissOtherPopups(except) {
    if (except !== "sort") closeSortMenu();
    if (except !== "filter") closeFilterMenu();
    if (except !== "ctx") closeContextMenu();
  }

  function parseNoteDate(iso) {
    if (!iso) return null;
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? null : d;
  }

  function startOfDay(d) {
    const x = new Date(d);
    x.setHours(0, 0, 0, 0);
    return x;
  }

  function relativeDayLabel(date) {
    const today = startOfDay(new Date());
    const that = startOfDay(date);
    const diff = Math.round((today - that) / 86400000);
    if (diff === 0) return "Сегодня";
    if (diff === 1) return "Вчера";
    if (diff < 7) return `${diff} дн. назад`;
    return date.toLocaleDateString("ru-RU", { day: "numeric", month: "short" });
  }

  function relativeTimeLabel(date) {
    const ms = Date.now() - date.getTime();
    if (ms < 60_000) return "только что";
    if (ms < 3_600_000) return `${Math.max(1, Math.floor(ms / 60_000))} мин. назад`;
    if (ms < 86_400_000) return `${Math.max(1, Math.floor(ms / 3_600_000))} ч. назад`;
    return relativeDayLabel(date);
  }

  function noteDateField(note, field) {
    return parseNoteDate(note[field]);
  }

  function sortNotes(notes) {
    const sorted = [...notes];
    if (state.sortBy === "title" || state.sortBy === "title-desc") {
      sorted.sort((a, b) => {
        const cmp = compareNames(a.title || noteLabel(a.name), b.title || noteLabel(b.name));
        return state.sortBy === "title-desc" ? -cmp : cmp;
      });
      return sorted;
    }
    const field = state.sortBy === "created" ? "created_at" : "modified_at";
    sorted.sort((a, b) => {
      const da = noteDateField(a, field)?.getTime() ?? 0;
      const db = noteDateField(b, field)?.getTime() ?? 0;
      return db - da;
    });
    return sorted;
  }

  function groupNotesByDate(notes) {
    const field = state.sortBy === "created" ? "created_at" : "modified_at";
    const buckets = new Map();

    for (const note of notes) {
      const date = noteDateField(note, field) || new Date(0);
      const today = startOfDay(new Date());
      const that = startOfDay(date);
      const diff = Math.round((today - that) / 86400000);
      let key = "older";
      let label = "Ранее";
      let order = 2;
      if (diff === 0) {
        key = "today";
        label = "Сегодня";
        order = 0;
      } else if (diff === 1) {
        key = "yesterday";
        label = "Вчера";
        order = 1;
      }
      if (!buckets.has(key)) {
        buckets.set(key, { key, label, order, notes: [] });
      }
      buckets.get(key).notes.push(note);
    }

    return [...buckets.values()].sort((a, b) => a.order - b.order);
  }

  function updateSortLabel() {
    if (!els.sortLabel) return;
    const opt = SORT_OPTIONS.find((o) => o.id === state.sortBy) || SORT_OPTIONS[0];
    els.sortLabel.textContent = opt.label;
    if (els.sortMenu) {
      syncMenuOptions(els.sortMenu, ".notes-sort__option", state.sortBy, "sort");
    }
  }

  function setSortBy(id) {
    if (!SORT_OPTIONS.some((o) => o.id === id)) return;
    state.sortBy = id;
    updateSortLabel();
    closeSortMenu();
    renderBento();
  }

  function initSortMenu() {
    if (!els.sortBtn || !els.sortMenu) return;
    sortDropdown = createDropdown({
      trigger: els.sortBtn,
      menu: els.sortMenu,
      optionSelector: ".notes-sort__option",
      dataKey: "sort",
      rootSelector: ".notes-sort",
      onOpen: () => dismissOtherPopups("sort"),
      onSelect: setSortBy,
    });
    updateSortLabel();
  }

  function filterDisplayLabel() {
    if (state.filterBy === "important") return "Важные";
    if (state.filterBy === "all") return "Все";
    const sec = state.sections.find((s) => s.id === state.filterBy);
    return sec?.name || "Все";
  }

  function filteredNotes() {
    let notes = state.notes;
    if (state.filterBy === "important") {
      notes = notes.filter((n) => n.important);
    } else if (state.filterBy !== "all") {
      notes = notes.filter((n) => n.section === state.filterBy);
    }
    if (state.searchQuery) {
      const names = new Set(Object.keys(state.searchSnippets));
      notes = notes.filter((n) => names.has(n.name));
    }
    return notes;
  }

  function stripSnippetMarkup(snippet) {
    return String(snippet || "").replace(/<\/?b>/gi, "").replace(/\s+/g, " ").trim();
  }

  let searchTimer = 0;

  function clearSearch() {
    state.searchQuery = "";
    state.searchHits = null;
    state.searchSnippets = {};
    searchControl?.clear();
  }

  async function runSearch(query, { quiet } = {}) {
    const q = query.trim();
    state.searchQuery = q;
    if (!q) {
      state.searchHits = null;
      state.searchSnippets = {};
      renderBento();
      return;
    }

    if (!quiet) els.bento.classList.add("is-loading");
    try {
      const hits = await api(`/api/search?q=${encodeURIComponent(q)}`);
      state.searchHits = hits;
      state.searchSnippets = {};
      for (const hit of hits) {
        if (hit.file) state.searchSnippets[hit.file] = hit.snippet || "";
      }
      if (state.view === "notes") renderBento();
    } catch (err) {
      showError(err);
    } finally {
      if (!quiet) els.bento.classList.remove("is-loading");
    }
  }

  function scheduleSearch(query) {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => runSearch(query), 250);
  }

  function initSearch() {
    if (!els.searchInput) return;
    searchControl = bindSearchInput(els.searchInput, {
      onSearch: scheduleSearch,
      onClear: () => {
        state.searchQuery = "";
        state.searchHits = null;
        state.searchSnippets = {};
        renderBento();
      },
    });
  }

  function updateFilterLabel() {
    if (!els.filterLabel) return;
    els.filterLabel.textContent = filterDisplayLabel();
  }

  function closeFilterMenu() {
    filterDropdown?.close();
    state.filterCreating = false;
  }

  function setFilterBy(id) {
    state.filterBy = id;
    state.filterCreating = false;
    localStorage.setItem(FILTER_STORAGE_KEY, id);
    updateFilterLabel();
    closeFilterMenu();
    renderBento();
  }

  function renderFilterMenu() {
    if (!els.filterMenu) return;
    els.filterMenu.innerHTML = "";

    for (const f of BUILTIN_FILTERS) {
      appendFilterOption(els.filterMenu, f);
    }

    for (const sec of state.sections) {
      appendFilterOption(els.filterMenu, { id: sec.id, label: sec.name, deletable: true });
    }

    if (state.filterCreating) {
      els.filterMenu.appendChild(createInlineForm({
        formClass: "notes-filter__form",
        inputClass: "notes-filter__input",
        cancelClass: "notes-filter__icon-btn",
        confirmClass: "notes-filter__icon-btn notes-filter__icon-btn--confirm",
        placeholder: "Название раздела",
        onConfirm: (name) => createSection(name).catch(showError),
        onCancel: () => {
          state.filterCreating = false;
          renderFilterMenu();
        },
      }));
      return;
    }

    els.filterMenu.appendChild(createAddMenuButton({
      className: "notes-filter__add",
      iconSrc: "assets/icon-plus.png",
      iconClass: "notes-filter__add-icon",
      label: "Новая",
      onClick: () => {
        state.filterCreating = true;
        renderFilterMenu();
      },
    }));
  }

  async function createSection(name) {
    const sec = await api("/api/sections", {
      method: "POST",
      body: JSON.stringify({ name }),
    });
    state.sections.push(sec);
    state.sections.sort((a, b) => a.name.localeCompare(b.name, "ru", { sensitivity: "base" }));
    state.filterBy = sec.id;
    state.filterCreating = false;
    localStorage.setItem(FILTER_STORAGE_KEY, sec.id);
    updateFilterLabel();
    closeFilterMenu();
    renderBento();
  }

  async function deleteSection(id) {
    await api(`/api/sections/${encodeURIComponent(id)}`, { method: "DELETE" });
    state.sections = state.sections.filter((s) => s.id !== id);
    if (state.filterBy === id) {
      state.filterBy = "all";
      localStorage.setItem(FILTER_STORAGE_KEY, "all");
      updateFilterLabel();
    }
    closeContextMenu();
    closeFilterMenu();
    await loadNotes();
    renderBento();
  }

  function openSectionContextMenu(e, sectionId) {
    if (!contextMenu) return;
    contextMenu.openAt(e, (menu) => {
      menu.appendChild(createContextAction({
        label: "Удалить",
        danger: true,
        onClick: () => deleteSection(sectionId).catch(showError),
      }));
    });
  }

  async function loadSections() {
    state.sections = await api("/api/sections");
    if (state.filterBy !== "all" && state.filterBy !== "important") {
      if (!state.sections.some((s) => s.id === state.filterBy)) {
        state.filterBy = "all";
        localStorage.setItem(FILTER_STORAGE_KEY, "all");
      }
    }
    updateFilterLabel();
  }

  function initFilterMenu() {
    if (!els.filterBtn || !els.filterMenu) return;
    filterDropdown = createDropdown({
      trigger: els.filterBtn,
      menu: els.filterMenu,
      optionSelector: ".notes-filter__option",
      dataKey: "filter",
      rootSelector: ".notes-filter",
      onOpen: () => {
        dismissOtherPopups("filter");
        renderFilterMenu();
        updateFilterLabel();
      },
      onSelect: setFilterBy,
    });
    updateFilterLabel();
  }

  function compareNames(a, b) {
    return a.localeCompare(b, "ru", { sensitivity: "base" });
  }

  function noteCardDate(note) {
    const mod = parseNoteDate(note.modified_at);
    return mod ? relativeTimeLabel(mod) : "";
  }

  function nextSSEBlock(buffer) {
    const lf = buffer.indexOf("\n\n");
    const crlf = buffer.indexOf("\r\n\r\n");
    if (lf === -1 && crlf === -1) return null;
    if (crlf !== -1 && (lf === -1 || crlf < lf)) {
      return { block: buffer.slice(0, crlf), rest: buffer.slice(crlf + 4) };
    }
    return { block: buffer.slice(0, lf), rest: buffer.slice(lf + 2) };
  }

  function parseSSEBlock(block) {
    let event = "message";
    let data = "";
    for (const raw of block.split(/\r?\n/)) {
      const line = raw.trimEnd();
      if (!line || line.startsWith(":")) continue;
      if (line.startsWith("event:")) event = line.slice(6).trim();
      else if (line.startsWith("data:")) data += line.slice(5).trim();
    }
    if (!data) return null;
    return { event, data: JSON.parse(data) };
  }

  async function readChatStream(res, onEvent) {
    if (!res.ok) {
      let message = res.statusText;
      try {
        const body = await res.json();
        if (body.error) message = body.error;
      } catch {
        // ignore
      }
      throw new Error(message);
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      let part;
      while ((part = nextSSEBlock(buffer))) {
        buffer = part.rest;
        const evt = parseSSEBlock(part.block);
        if (!evt) continue;

        if (evt.event === "status") {
          onEvent(evt.data);
        } else if (evt.event === "done") {
          return evt.data;
        } else if (evt.event === "error") {
          throw new Error(evt.data.error || "ollama unavailable");
        }
      }
    }

    throw new Error("stream ended without result");
  }

  async function chatStream(messages, onEvent) {
    const res = await fetch("/api/chat", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "text/event-stream",
      },
      cache: "no-store",
      body: JSON.stringify({ messages }),
    });
    return readChatStream(res, onEvent);
  }

  function upsertNote(file, title) {
    if (!file) return;
    const i = state.notes.findIndex((n) => n.name === file);
    const now = new Date().toISOString();
    if (i === -1) {
      state.notes.push({
        name: file,
        title: title || noteLabel(file),
        preview: "",
        created_at: now,
        modified_at: now,
      });
    } else {
      state.notes[i].modified_at = now;
      if (title) state.notes[i].title = title;
    }
    renderBento();
  }

  function removeNote(file) {
    if (!file) return;
    state.notes = state.notes.filter((n) => n.name !== file);
    if (state.active?.name === file) {
      state.active = null;
      clearReader();
    }
    renderBento();
  }

  function daysLeftLabel(days) {
    if (days <= 0) return "сегодня";
    return `ещё ${days} дн.`;
  }

  function setView(view) {
    state.view = view;
    els.toolbarTitle.textContent = view === "trash" ? "Корзина" : "Заметки";
    els.trashBtn.classList.toggle("is-active", view === "trash");
  }

  function notesRoute() {
    return location.pathname === "/notes" || location.hash === "#notes";
  }

  function applyScreen(screen) {
    document.body.dataset.screen = screen;
    if (screen === "home") {
      clearReader();
      return;
    }
    setView("notes");
    Promise.all([loadSections(), loadNotes()]).catch(showError);
  }

  function goHome() {
    if (notesRoute()) {
      history.pushState(null, "", "/");
    }
    applyScreen("home");
  }

  function goNotes() {
    if (location.pathname !== "/notes") {
      history.pushState(null, "", "/notes");
    }
    applyScreen("notes");
  }

  async function api(path, options) {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json" },
      ...options,
    });

    if (!res.ok) {
      let message = res.statusText;
      try {
        const body = await res.json();
        if (body.error) message = body.error;
      } catch {
        // ignore
      }
      throw new Error(message);
    }

    if (res.status === 204) return null;
    return res.json();
  }

  function renderBento() {
    els.bento.innerHTML = "";

    if (state.view === "trash") {
      renderTrashBento();
      return;
    }

    if (state.notes.length === 0) {
      const empty = document.createElement("div");
      empty.className = "notes-bento__empty";
      empty.textContent = "Нет заметок";
      els.bento.appendChild(empty);
      return;
    }

    const visible = filteredNotes();
    if (visible.length === 0) {
      const empty = document.createElement("div");
      empty.className = "notes-bento__empty";
      empty.textContent = state.searchQuery ? "Ничего не найдено" : "Нет заметок в этом разделе";
      els.bento.appendChild(empty);
      return;
    }

    const sorted = sortNotes(visible);
    if (state.searchQuery || state.sortBy === "title" || state.sortBy === "title-desc") {
      const label = state.searchQuery ? "Результаты" : "Заметки";
      els.bento.appendChild(createNotesGroup(label, sorted.length, sorted.map(createNoteCard)));
      return;
    }

    for (const group of groupNotesByDate(sorted)) {
      els.bento.appendChild(createNotesGroup(group.label, group.notes.length, group.notes.map(createNoteCard)));
    }
  }

  function createNotesGroup(label, count, cards) {
    const group = document.createElement("div");
    group.className = "notes-group";

    const head = document.createElement("div");
    head.className = "notes-group__head";

    const title = document.createElement("h2");
    title.className = "notes-group__title";
    title.textContent = label;

    const dot = document.createElement("span");
    dot.className = "notes-group__dot";
    dot.textContent = "·";

    const meta = document.createElement("span");
    meta.className = "notes-group__count";
    meta.textContent = `${count} ${noteCountLabel(count)}`;

    head.append(title, dot, meta);

    const grid = document.createElement("div");
    grid.className = "notes-group__grid";
    for (const card of cards) grid.appendChild(card);

    group.append(head, grid);
    return group;
  }

  function noteCountLabel(n) {
    const mod10 = n % 10;
    const mod100 = n % 100;
    if (mod10 === 1 && mod100 !== 11) return "заметка";
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return "заметки";
    return "заметок";
  }

  function createNoteCard(note) {
    const snippet = state.searchSnippets[note.name];
    const preview = snippet
      ? stripSnippetMarkup(snippet) || cardPreview(note.preview)
      : cardPreview(note.preview);
    return createNoteCardComponent({
      path: note.name,
      title: note.title || noteLabel(note.name),
      preview,
      date: noteCardDate(note),
      isActive: state.active?.name === note.name,
      onClick: () => selectNote(note.name),
      onContextMenu: (e) => openNoteContextMenu(e, note.name),
    });
  }

  function createTrashCard(item) {
    return createNoteCardComponent({
      path: item.name,
      title: item.title || noteLabel(item.name),
      preview: cardPreview(item.content),
      date: daysLeftLabel(item.days_left),
      isActive: state.activeTrash?.id === item.id,
      onClick: () => selectTrashItem(item.id),
    });
  }

  function renderTrashBento() {
    if (state.trash.length === 0) {
      const empty = document.createElement("div");
      empty.className = "notes-bento__empty";
      empty.textContent = "Корзина пуста";
      els.bento.appendChild(empty);
      return;
    }

    const cards = state.trash.map(createTrashCard);
    els.bento.appendChild(createNotesGroup("Корзина", cards.length, cards));
  }

  let noteEditor = null;

  async function saveNoteContent(name, content) {
    const note = await api(`/api/notes/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify({ content }),
    });
    state.active = note;
    const i = state.notes.findIndex((n) => n.name === name);
    if (i !== -1) {
      state.notes[i].modified_at = note.modified_at;
      state.notes[i].preview = (content || "").replace(/\s+/g, " ").trim().slice(0, 120);
    }
  }

  function clearReader() {
    state.active = null;
    state.activeTrash = null;
    els.readerPanel.hidden = true;
    els.readerEmpty.hidden = false;
    if (els.readerFoot) els.readerFoot.hidden = true;
    if (noteEditor) {
      noteEditor.setContent("");
      noteEditor.setReadOnly(false);
    }
  }

  function openNoteEditor(content, { readOnly = false } = {}) {
    els.readerEmpty.hidden = true;
    els.readerPanel.hidden = false;
    if (els.readerFoot) els.readerFoot.hidden = !(readOnly && state.view === "trash");
    noteEditor.setReadOnly(readOnly);
    noteEditor.setContent(content || "");
    if (!readOnly) noteEditor.focus();
  }

  function openNoteReader(note) {
    openNoteEditor(note.content, { readOnly: false });
  }

  function openTrashReader(item) {
    openNoteEditor(item.content, { readOnly: true });
  }

  async function loadNotes() {
    els.bento.classList.add("is-loading");
    try {
      state.notes = await api("/api/notes");
      if (state.view === "notes") {
        if (state.searchQuery) await runSearch(state.searchQuery, { quiet: true });
        else renderBento();
      }
    } finally {
      els.bento.classList.remove("is-loading");
    }
  }

  async function loadTrash() {
    els.bento.classList.add("is-loading");
    try {
      state.trash = await api("/api/trash");
      if (state.view === "trash") renderBento();
    } finally {
      els.bento.classList.remove("is-loading");
    }
  }

  async function selectNote(name) {
    try {
      state.active = await api(`/api/notes/${encodeURIComponent(name)}`);
      state.activeTrash = null;
      renderBento();
      openNoteReader(state.active);
    } catch (err) {
      showError(err);
    }
  }

  async function selectTrashItem(id) {
    try {
      state.activeTrash = await api(`/api/trash/${encodeURIComponent(id)}`);
      state.active = null;
      renderBento();
      openTrashReader(state.activeTrash);
    } catch (err) {
      showError(err);
    }
  }

  async function trashNote(name) {
    if (!name) return;
    await api("/api/trash", {
      method: "POST",
      body: JSON.stringify({ name }),
    });
    removeNote(name);
    if (state.view === "trash") await loadTrash();
  }

  async function moveNoteTo(name, target) {
    let patch;
    if (target === "important") {
      patch = { important: true, section: "" };
    } else {
      patch = { section: target, important: false };
    }
    const note = await api(`/api/notes/${encodeURIComponent(name)}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    });
    const i = state.notes.findIndex((n) => n.name === name);
    if (i !== -1) {
      state.notes[i] = {
        ...state.notes[i],
        section: note.section || "",
        important: !!note.important,
        modified_at: note.modified_at,
      };
    }
    if (state.active?.name === name) {
      state.active = note;
      openNoteReader(note);
    }
    renderBento();
  }

  function openNoteContextMenu(e, noteName) {
    if (state.view === "trash" || !contextMenu) return;
    dismissOtherPopups("ctx");
    const note = state.notes.find((n) => n.name === noteName);
    contextMenu.openAt(e, (menu) => {
      menu.appendChild(createSubmenuRow({
        label: "Переместить",
        items: sectionMoveTargets().map((t) => ({
          id: t.id,
          label: t.label,
          active: noteMatchesSection(note, t.id),
        })),
        onSelect: (id) => {
          closeContextMenu();
          moveNoteTo(noteName, id).catch(showError);
        },
      }));
      menu.appendChild(createContextAction({
        label: "В корзину",
        danger: true,
        onClick: () => {
          closeContextMenu();
          trashNote(noteName).catch(showError);
        },
      }));
    });
  }

  function initContextMenu() {
    if (!els.ctxRoot || !els.ctxMenu) return;
    contextMenu = createContextMenu({ root: els.ctxRoot, menu: els.ctxMenu });
    contextMenu.mount();
  }

  function initPopupDismiss() {
    registerPopupDismiss([
      { rootSelector: ".notes-sort", close: closeSortMenu },
      { rootSelector: ".notes-filter", close: closeFilterMenu },
      { rootSelector: ".notes-ctx", close: closeContextMenu },
    ]);
  }

  async function restoreActiveTrash() {
    const id = state.activeTrash?.id;
    if (!id) return;
    els.readerAction.disabled = true;
    try {
      const note = await api(`/api/trash/${encodeURIComponent(id)}/restore`, {
        method: "POST",
      });
      state.activeTrash = null;
      state.trash = state.trash.filter((item) => item.id !== id);
      clearReader();
      if (state.view === "trash") renderBento();
      await loadNotes();
      if (note?.name) {
        state.active = note;
        renderBento();
        openNoteReader(note);
      }
    } catch (err) {
      showError(err);
    } finally {
      els.readerAction.disabled = false;
    }
  }

  async function showTrashView() {
    setView("trash");
    clearSearch();
    state.active = null;
    clearReader();
    renderBento();
    try {
      await loadTrash();
    } catch (err) {
      showError(err);
    }
  }

  async function showNotesView() {
    setView("notes");
    state.activeTrash = null;
    clearReader();
    renderBento();
    try {
      await loadNotes();
    } catch (err) {
      showError(err);
    }
  }

  function uniqueName(base) {
    const existing = new Set(state.notes.map((n) => n.name));
    if (!existing.has(base)) return base;
    const stem = base.replace(/\.md$/, "");
    for (let i = 2; ; i++) {
      const candidate = `${stem}-${i}.md`;
      if (!existing.has(candidate)) return candidate;
    }
  }

  async function createNote() {
    els.addBtn.disabled = true;
    try {
      const name = uniqueName("novaya-zametka.md");
      const payload = { name, content: "" };
      if (state.filterBy === "important") payload.important = true;
      else if (state.filterBy !== "all") payload.section = state.filterBy;
      const note = await api("/api/notes", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      setView("notes");
      await loadNotes();
      state.active = note || { name, content: "" };
      renderBento();
      openNoteReader(state.active);
    } catch (err) {
      showError(err);
    } finally {
      els.addBtn.disabled = false;
    }
  }

  function showError(err) {
    els.bento.innerHTML = "";
    const item = document.createElement("div");
    item.className = "notes-bento__empty notes-bento__empty--error";
    item.textContent = err.message;
    els.bento.appendChild(item);
  }

  /* Legacy chat — preserved for future Cloud section */

  function appendSearchFacts(facts, matches) {
    const group = document.createElement("div");
    group.className = "facts__group";

    const label = document.createElement("div");
    label.className = "facts__label facts__label--found";
    if (!matches.length) {
      label.textContent = "Ничего не найдено";
      group.appendChild(label);
      facts.appendChild(group);
      return;
    }

    label.textContent = `Найдено · ${matches.length}`;
    group.appendChild(label);

    const chips = document.createElement("div");
    chips.className = "facts__chips";
    for (const hit of matches) {
      const chip = document.createElement("span");
      chip.className = "facts__chip";
      chip.textContent = hit.title || noteLabel(hit.file);
      chip.title = hit.file;
      chips.appendChild(chip);
    }
    group.appendChild(chips);
    facts.appendChild(group);
  }

  function createFactsGroup(kind, files) {
    if (!files?.length) return null;

    const group = document.createElement("div");
    group.className = "facts__group";

    const label = document.createElement("div");
    label.className = `facts__label facts__label--${kind}`;
    const labels = { created: "Создано", updated: "Обновлено" };
    label.textContent = `${labels[kind] || kind} · ${files.length}`;
    group.appendChild(label);

    const chips = document.createElement("div");
    chips.className = "facts__chips";
    for (const file of files) {
      const chip = document.createElement("span");
      chip.className = "facts__chip";
      chip.textContent = noteLabel(file);
      chip.title = file;
      chips.appendChild(chip);
    }
    group.appendChild(chips);

    return group;
  }

  function createAssistantMessage(msg) {
    const wrap = document.createElement("article");
    wrap.className = "message message--assistant message--enter";

    const content = document.createElement("div");
    content.className = "message__content";
    renderMarkdown(content, msg.content || "");
    wrap.appendChild(content);

    if (msg.created?.length || msg.updated?.length || msg.searched) {
      const facts = document.createElement("div");
      facts.className = "message__facts message__facts--enter";

      const header = document.createElement("div");
      header.className = "facts__header";
      header.textContent = "На диске";
      facts.appendChild(header);

      if (msg.searched) {
        appendSearchFacts(facts, msg.matches || []);
      }
      const created = createFactsGroup("created", msg.created);
      if (created) facts.appendChild(created);
      const updated = createFactsGroup("updated", msg.updated);
      if (updated) facts.appendChild(updated);

      wrap.appendChild(facts);
    }

    return wrap;
  }

  function createPendingMessage() {
    const wrap = document.createElement("article");
    wrap.className = "message message--pending message--enter";
    wrap.setAttribute("aria-live", "polite");
    wrap.setAttribute("aria-label", "Думаю");

    const indicator = document.createElement("div");
    indicator.className = "typing-indicator";
    indicator.setAttribute("aria-hidden", "true");
    for (let i = 0; i < 3; i++) {
      indicator.appendChild(document.createElement("span"));
    }
    wrap.appendChild(indicator);

    const label = document.createElement("span");
    label.className = "message__pending-text";
    label.textContent = "Думаю";
    wrap.appendChild(label);

    return wrap;
  }

  function renderChat() {
    if (!els.messages) return;
    els.messages.innerHTML = "";

    if (state.chat.length === 0 && !state.pending) {
      const hint = document.createElement("p");
      hint.className = "chat__hint";
      hint.textContent = "Напишите сообщение — AI может создавать и редактировать заметки";
      els.messages.appendChild(hint);
      return;
    }

    for (const msg of state.chat) {
      if (msg.role === "assistant") {
        els.messages.appendChild(createAssistantMessage(msg));
        continue;
      }

      const el = document.createElement("p");
      el.className = `message message--${msg.role} message--enter`;
      el.textContent = msg.content;
      els.messages.appendChild(el);
    }

    if (state.pending) {
      els.messages.appendChild(createPendingMessage());
    }

    els.messages.scrollTop = els.messages.scrollHeight;
  }

  function chatHistory() {
    return state.chat.filter((m) => m.role === "user" || m.role === "assistant");
  }

  async function sendMessage(text) {
    if (state.sending) return;

    state.sending = true;
    state.pending = true;
    els.chatInput.disabled = true;
    els.chatSubmit.disabled = true;

    state.chat.push({ role: "user", content: text });
    renderChat();

    try {
      const res = await chatStream(chatHistory(), (phase) => {
        if ((phase.kind === "created" || phase.kind === "updated") && phase.file) {
          upsertNote(phase.file, phase.title);
        }
      });

      state.chat.push({
        role: "assistant",
        content: res.content,
        created: res.created || [],
        updated: res.updated || [],
        searched: !!res.searched,
        matches: res.matches || [],
      });

      if (res.notes_changed) {
        await loadNotes();
      }
    } catch (err) {
      state.chat.push({ role: "error", content: err.message });
    } finally {
      state.sending = false;
      state.pending = false;
      els.chatInput.disabled = false;
      els.chatSubmit.disabled = false;
      renderChat();
      els.chatInput.focus();
    }
  }

  /* Events */

  if (els.notesCard) els.notesCard.addEventListener("click", goNotes);
  if (els.homeBtn) els.homeBtn.addEventListener("click", goHome);
  window.addEventListener("popstate", () => {
    applyScreen(notesRoute() ? "notes" : "home");
  });

  if (els.addBtn) els.addBtn.addEventListener("click", () => createNote());
  if (els.trashBtn) {
    els.trashBtn.addEventListener("click", () => {
      if (state.view === "trash") showNotesView();
      else showTrashView();
    });
  }
  initSortMenu();
  initFilterMenu();
  initSearch();
  initContextMenu();
  initPopupDismiss();
  noteEditor = createMarkdownEditor({
    panel: els.readerPanel,
    onChange: (content) => {
      if (!state.active?.name || state.view === "trash") return;
      saveNoteContent(state.active.name, content).catch(showError);
    },
  });
  if (els.readerAction) {
    els.readerAction.addEventListener("click", () => {
      if (state.view !== "trash" || !state.activeTrash?.id) return;
      restoreActiveTrash();
    });
  }

  if (els.chatForm) {
    els.chatForm.addEventListener("submit", (e) => {
      e.preventDefault();
      const text = els.chatInput.value.trim();
      if (!text || state.sending) return;
      els.chatInput.value = "";
      sendMessage(text);
    });
  }

  if (els.chatInput) {
    els.chatInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        els.chatForm.requestSubmit();
      }
    });
  }

  (function initNotesSplit() {
    const body = document.querySelector(".notes-body");
    const splitter = document.querySelector(".notes-splitter");
    if (!body || !splitter) return;

    const STORAGE_KEY = "notes-split-width";
    const MIN = 320;
    const SPLITTER = 6;
    let dragging = false;

    function splitEnabled() {
      return window.matchMedia("(min-width: 1101px)").matches;
    }

    function clamp(width) {
      const max = Math.max(MIN, body.clientWidth - MIN - SPLITTER);
      return Math.min(Math.max(width, MIN), max);
    }

    function currentWidth() {
      const raw = getComputedStyle(body).getPropertyValue("--notes-split").trim();
      const n = parseInt(raw, 10);
      return Number.isFinite(n) ? n : 1000;
    }

    function setWidth(px) {
      body.style.setProperty("--notes-split", `${clamp(px)}px`);
      localStorage.setItem(STORAGE_KEY, String(clamp(px)));
    }

    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      const n = parseInt(saved, 10);
      if (Number.isFinite(n)) setWidth(n);
    }

    function stopDrag() {
      if (!dragging) return;
      dragging = false;
      splitter.classList.remove("is-dragging");
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    }

    splitter.addEventListener("mousedown", (e) => {
      if (!splitEnabled()) return;
      e.preventDefault();
      dragging = true;
      splitter.classList.add("is-dragging");
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
    });

    window.addEventListener("mousemove", (e) => {
      if (!dragging) return;
      const rect = body.getBoundingClientRect();
      setWidth(e.clientX - rect.left);
    });

    window.addEventListener("mouseup", stopDrag);
    window.addEventListener("blur", stopDrag);

    splitter.addEventListener("keydown", (e) => {
      if (!splitEnabled()) return;
      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
      e.preventDefault();
      const step = e.shiftKey ? 48 : 16;
      setWidth(currentWidth() + (e.key === "ArrowRight" ? step : -step));
    });

    window.addEventListener("resize", () => {
      if (splitEnabled()) setWidth(currentWidth());
    });
  })();

  renderChat();
  if (notesRoute()) applyScreen("notes");
