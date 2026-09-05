(() => {
  "use strict";

  const state = {
    notes: [],
    trash: [],
    view: "notes",
    active: null,
    activeTrash: null,
    chat: [],
    sending: false,
    pending: false,
    openFolders: new Set(),
  };

  const els = {
    // Notes workspace
    bento: document.querySelector(".notes-bento"),
    toolbarTitle: document.querySelector(".notes-toolbar__title"),
    homeBtn: document.querySelector(".notes-toolbar__home"),
    addBtn: document.querySelector(".notes-toolbar__add"),
    trashBtn: document.querySelector(".notes-toolbar__trash"),
    notesCard: document.querySelector("[data-open=\"notes\"]"),
    readerEmpty: document.querySelector(".notes-reader__empty"),
    readerPanel: document.querySelector(".notes-reader__panel"),
    readerTitle: document.querySelector(".notes-reader__title"),
    readerMeta: document.querySelector(".notes-reader__meta"),
    readerBody: document.querySelector(".notes-reader__body"),
    readerAction: document.querySelector(".notes-reader__action"),
    // Legacy (mummified — preserved for future Cloud section)
    list: document.querySelector(".sidebar__list"),
    label: document.querySelector(".sidebar__label"),
    messages: document.querySelector(".chat__messages"),
    chatForm: document.querySelector(".chat__input"),
    chatInput: document.querySelector(".chat__input textarea"),
    chatSubmit: document.querySelector(".chat__input button"),
    modal: document.querySelector(".note-modal"),
    modalTitle: document.querySelector(".note-modal__title"),
    modalBody: document.querySelector(".note-modal__body"),
    modalClose: document.querySelector(".note-modal__close"),
    modalAction: document.querySelector(".note-modal__action"),
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

  function compareNames(a, b) {
    return a.localeCompare(b, "ru", { sensitivity: "base" });
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
    if (!file || state.notes.some((n) => n.name === file)) return;
    state.notes.push({ name: file, title: title || noteLabel(file), preview: "" });
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
    loadNotes().catch(showError);
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

    const sorted = [...state.notes].sort((a, b) => compareNames(a.title, b.title));
    for (const note of sorted) {
      els.bento.appendChild(createNoteCard(note));
    }
  }

  function createNoteCard(note) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "notes-card";
    if (state.active?.name === note.name) btn.classList.add("is-active");
    btn.title = note.name;

    const title = document.createElement("span");
    title.className = "notes-card__title";
    title.textContent = note.title || noteLabel(note.name);
    btn.appendChild(title);

    const preview = document.createElement("span");
    preview.className = "notes-card__preview";
    preview.textContent = cardPreview(note.preview);
    btn.appendChild(preview);

    btn.addEventListener("click", () => selectNote(note.name));
    return btn;
  }

  function renderTrashBento() {
    if (state.trash.length === 0) {
      const empty = document.createElement("div");
      empty.className = "notes-bento__empty";
      empty.textContent = "Корзина пуста";
      els.bento.appendChild(empty);
      return;
    }

    for (const item of state.trash) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "notes-card";
      if (state.activeTrash?.id === item.id) btn.classList.add("is-active");
      btn.title = item.name;

      const title = document.createElement("span");
      title.className = "notes-card__title";
      title.textContent = item.title || noteLabel(item.name);
      btn.appendChild(title);

      const preview = document.createElement("span");
      preview.className = "notes-card__preview";
      preview.textContent = cardPreview(item.content);
      btn.appendChild(preview);

      btn.addEventListener("click", () => selectTrashItem(item.id));
      els.bento.appendChild(btn);
    }
  }

  function escapeHtml(text) {
    return text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function stripToolMarkup(text) {
    return text
      .replace(/<tool_call>[\s\S]*?<\/tool_call>/gi, "")
      .replace(/<\/?tool_call>/gi, "")
      .trim();
  }

  function renderMarkdown(el, text) {
    const fences = [];
    let html = escapeHtml(stripToolMarkup(text)).replace(/```[^\n]*\n?([\s\S]*?)```/g, (_, code) => {
      fences.push(`<pre class="md-pre"><code>${code.replace(/\n$/, "")}</code></pre>`);
      return `\0${fences.length - 1}\0`;
    });

    html = html.replace(/`([^`]+)`/g, '<code class="md-code">$1</code>');
    html = html.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    html = html.replace(/__([^_]+)__/g, "<strong>$1</strong>");
    html = html.replace(/(^|[\s(])\*([^*\n]+)\*(?=[\s).,]|$)/gm, "$1<em>$2</em>");
    html = html.replace(/^#{1,3}[ \t]+(.+)$/gm, '<span class="md-heading">$1</span>');
    html = html.replace(/\0(\d+)\0/g, (_, i) => fences[Number(i)]);

    el.innerHTML = html;
  }

  function clearReader() {
    state.active = null;
    state.activeTrash = null;
    els.readerPanel.hidden = true;
    els.readerEmpty.hidden = false;
  }

  function showReader(title, content, meta, actionLabel, danger) {
    const empty = !content || !String(content).trim();
    els.readerTitle.textContent = title;
    els.readerMeta.textContent = meta || "";
    els.readerMeta.hidden = !meta;

    if (empty) {
      els.readerBody.textContent = "Пустая заметка";
      els.readerBody.classList.add("notes-reader__body--empty");
    } else {
      renderMarkdown(els.readerBody, content);
      els.readerBody.classList.remove("notes-reader__body--empty");
    }

    els.readerAction.textContent = actionLabel;
    els.readerAction.classList.toggle("notes-reader__action--danger", !!danger);
    els.readerEmpty.hidden = true;
    els.readerPanel.hidden = false;
  }

  function openNoteReader(note) {
    const summary = state.notes.find((n) => n.name === note.name);
    const folder = noteFolder(note.name);
    showReader(
      summary?.title || noteLabel(note.name),
      note.content,
      folder,
      "В корзину",
      true,
    );
  }

  function openTrashReader(item) {
    showReader(
      item.title || noteLabel(item.name),
      item.content,
      daysLeftLabel(item.days_left),
      "Восстановить",
      false,
    );
  }

  async function loadNotes() {
    els.bento.classList.add("is-loading");
    try {
      state.notes = await api("/api/notes");
      if (state.view === "notes") renderBento();
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

  async function trashActiveNote() {
    const name = state.active?.name;
    if (!name) return;
    els.readerAction.disabled = true;
    try {
      await api("/api/trash", {
        method: "POST",
        body: JSON.stringify({ name }),
      });
      removeNote(name);
      if (state.view === "trash") await loadTrash();
    } catch (err) {
      showError(err);
    } finally {
      els.readerAction.disabled = false;
    }
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
      const note = await api("/api/notes", {
        method: "POST",
        body: JSON.stringify({ name, content: "" }),
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
  if (els.readerAction) {
    els.readerAction.addEventListener("click", () => {
      if (state.activeTrash) restoreActiveTrash();
      else trashActiveNote();
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

  renderChat();
  if (notesRoute()) applyScreen("notes");
})();
