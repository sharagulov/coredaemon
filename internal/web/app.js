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
    list: document.querySelector(".sidebar__list"),
    label: document.querySelector(".sidebar__label"),
    homeBtn: document.querySelector(".sidebar__home"),
    notesCard: document.querySelector("[data-open=\"notes\"]"),
    addBtn: document.querySelector(".sidebar__add"),
    trashBtn: document.querySelector(".sidebar__trash"),
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

  function revealFolders(file) {
    const parts = String(file || "").split("/");
    let path = "";
    for (let i = 0; i < parts.length - 1; i++) {
      path = path ? `${path}/${parts[i]}` : parts[i];
      state.openFolders.add(path);
    }
  }

  function toggleFolder(path) {
    if (state.openFolders.has(path)) state.openFolders.delete(path);
    else state.openFolders.add(path);
    renderList();
  }

  function compareNames(a, b) {
    return a.localeCompare(b, "ru", { sensitivity: "base" });
  }

  function buildNoteTree(notes) {
    const root = { folders: new Map(), files: [] };
    const sorted = [...notes].sort((a, b) => compareNames(a.name, b.name));
    for (const note of sorted) {
      const parts = note.name.split("/");
      let node = root;
      for (let i = 0; i < parts.length - 1; i++) {
        const seg = parts[i];
        if (!node.folders.has(seg)) {
          node.folders.set(seg, { folders: new Map(), files: [] });
        }
        node = node.folders.get(seg);
      }
      node.files.push(note);
    }
    return root;
  }

  function glyph(kind) {
    const el = document.createElement("span");
    el.className = `sidebar__glyph sidebar__glyph--${kind}`;
    el.setAttribute("aria-hidden", "true");
    if (kind === "chevron") {
      el.innerHTML = '<svg viewBox="0 0 16 16"><path d="M6 4.5 10.5 8 6 11.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>';
    } else if (kind === "folder") {
      el.innerHTML = '<svg viewBox="0 0 16 16"><path d="M2.5 5.2c0-.7.6-1.2 1.2-1.2h2.2l.8 1.1h5.6c.7 0 1.2.6 1.2 1.2v5.3c0 .7-.5 1.2-1.2 1.2h-8.6c-.7 0-1.2-.5-1.2-1.2z" fill="none" stroke="currentColor" stroke-width="1.3"/></svg>';
    } else {
      el.innerHTML = '<svg viewBox="0 0 16 16"><path d="M4.5 2.5h4.2L11.5 5.3v8.2h-7z" fill="none" stroke="currentColor" stroke-width="1.3"/><path d="M8.7 2.5v2.8h2.8" fill="none" stroke="currentColor" stroke-width="1.3"/></svg>';
    }
    return el;
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
    revealFolders(file);
    renderList();
  }

  function removeNote(file) {
    if (!file) return;
    state.notes = state.notes.filter((n) => n.name !== file);
    if (state.active?.name === file) {
      state.active = null;
      if (els.modal.open) closeNoteModal();
    }
    renderList();
  }

  function daysLeftLabel(days) {
    if (days <= 0) return "сегодня";
    return `ещё ${days} дн.`;
  }

  function setView(view) {
    state.view = view;
    els.label.textContent = view === "trash" ? "Корзина" : "Заметки";
    els.trashBtn.classList.toggle("is-active", view === "trash");
  }

  function applyScreen(screen) {
    document.body.dataset.screen = screen;
    if (screen === "home") {
      if (els.modal.open) closeNoteModal();
      return;
    }
    setView("notes");
    loadNotes().catch(showError);
  }

  function goHome() {
    if (location.hash === "#notes") {
      history.pushState(null, "", location.pathname + location.search);
    }
    applyScreen("home");
  }

  function goNotes() {
    if (location.hash !== "#notes") {
      history.pushState(null, "", "#notes");
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

  function renderList() {
    els.list.innerHTML = "";
    if (state.view === "trash") {
      renderTrash();
      return;
    }
    renderNotes();
  }

  function renderNotes() {
    if (state.notes.length === 0) {
      const empty = document.createElement("li");
      empty.className = "sidebar__empty";
      empty.textContent = "Нет заметок";
      els.list.appendChild(empty);
      return;
    }

    renderNoteTree(els.list, buildNoteTree(state.notes), "", 0);
  }

  function renderTrash() {
    if (state.trash.length === 0) {
      const empty = document.createElement("li");
      empty.className = "sidebar__empty";
      empty.textContent = "Корзина пуста";
      els.list.appendChild(empty);
      return;
    }

    for (const item of state.trash) {
      const row = document.createElement("li");
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "sidebar__row sidebar__file sidebar__file--trash";
      btn.title = item.name;
      if (state.activeTrash?.id === item.id) btn.classList.add("is-active");
      btn.appendChild(glyph("file"));

      const label = document.createElement("span");
      label.className = "sidebar__row-label";
      label.textContent = item.title || noteLabel(item.name);
      btn.appendChild(label);

      const meta = document.createElement("span");
      meta.className = "sidebar__trash-meta";
      meta.textContent = daysLeftLabel(item.days_left);
      btn.appendChild(meta);
      btn.addEventListener("click", () => selectTrashItem(item.id));
      row.appendChild(btn);
      els.list.appendChild(row);
    }
  }

  function renderNoteTree(parent, node, prefix, depth) {
    const folderNames = [...node.folders.keys()].sort(compareNames);
    for (const name of folderNames) {
      const path = prefix ? `${prefix}/${name}` : name;
      const open = state.openFolders.has(path);
      const item = document.createElement("li");

      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "sidebar__row sidebar__folder";
      if (open) btn.classList.add("is-open");
      btn.style.paddingLeft = `${0.35 + depth * 0.7}rem`;
      btn.title = path;
      btn.setAttribute("aria-expanded", open ? "true" : "false");
      btn.append(glyph("chevron"), glyph("folder"));
      const label = document.createElement("span");
      label.className = "sidebar__row-label";
      label.textContent = name;
      btn.appendChild(label);
      btn.addEventListener("click", () => toggleFolder(path));
      item.appendChild(btn);

      if (open) {
        const nested = document.createElement("ul");
        nested.className = "sidebar__tree";
        renderNoteTree(nested, node.folders.get(name), path, depth + 1);
        item.appendChild(nested);
      }

      parent.appendChild(item);
    }

    for (const note of node.files) {
      const item = document.createElement("li");
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "sidebar__row sidebar__file";
      btn.style.paddingLeft = `${0.35 + depth * 0.7}rem`;
      btn.title = note.name;
      if (state.active?.name === note.name) {
        btn.classList.add("is-active");
      }
      btn.appendChild(glyph("file"));
      const label = document.createElement("span");
      label.className = "sidebar__row-label";
      label.textContent = note.title;
      btn.appendChild(label);
      btn.addEventListener("click", () => selectNote(note.name));
      item.appendChild(btn);
      parent.appendChild(item);
    }
  }

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

  async function loadNotes() {
    els.list.classList.add("is-loading");
    try {
      state.notes = await api("/api/notes");
      if (state.view === "notes") renderList();
    } finally {
      els.list.classList.remove("is-loading");
    }
  }

  async function loadTrash() {
    els.list.classList.add("is-loading");
    try {
      state.trash = await api("/api/trash");
      if (state.view === "trash") renderList();
    } finally {
      els.list.classList.remove("is-loading");
    }
  }

  function fillModal(title, content, actionLabel, danger) {
    els.modalTitle.textContent = title;
    const empty = !content || !String(content).trim();
    els.modalBody.textContent = empty ? "Пустая заметка" : content;
    els.modalBody.classList.toggle("note-modal__body--empty", empty);
    els.modalAction.textContent = actionLabel;
    els.modalAction.classList.toggle("note-modal__action--danger", !!danger);
    els.modal.showModal();
  }

  function openNoteModal(note) {
    const summary = state.notes.find((n) => n.name === note.name);
    fillModal(summary?.title || note.name, note.content, "В корзину", true);
  }

  function openTrashModal(item) {
    fillModal(item.title || noteLabel(item.name), item.content, "Восстановить", false);
  }

  function closeNoteModal() {
    els.modal.close();
  }

  async function selectNote(name) {
    try {
      revealFolders(name);
      state.active = await api(`/api/notes/${encodeURIComponent(name)}`);
      state.activeTrash = null;
      renderList();
      openNoteModal(state.active);
    } catch (err) {
      showError(err);
    }
  }

  async function selectTrashItem(id) {
    try {
      state.activeTrash = await api(`/api/trash/${encodeURIComponent(id)}`);
      state.active = null;
      renderList();
      openTrashModal(state.activeTrash);
    } catch (err) {
      showError(err);
    }
  }

  async function trashActiveNote() {
    const name = state.active?.name;
    if (!name) return;
    els.modalAction.disabled = true;
    try {
      await api("/api/trash", {
        method: "POST",
        body: JSON.stringify({ name }),
      });
      closeNoteModal();
      removeNote(name);
      if (state.view === "trash") await loadTrash();
    } catch (err) {
      showError(err);
    } finally {
      els.modalAction.disabled = false;
    }
  }

  async function restoreActiveTrash() {
    const id = state.activeTrash?.id;
    if (!id) return;
    els.modalAction.disabled = true;
    try {
      const note = await api(`/api/trash/${encodeURIComponent(id)}/restore`, {
        method: "POST",
      });
      closeNoteModal();
      state.activeTrash = null;
      state.trash = state.trash.filter((item) => item.id !== id);
      if (state.view === "trash") renderList();
      await loadNotes();
      if (note?.name) {
        revealFolders(note.name);
        state.active = note;
      }
    } catch (err) {
      showError(err);
    } finally {
      els.modalAction.disabled = false;
    }
  }

  async function showTrashView() {
    setView("trash");
    state.active = null;
    renderList();
    try {
      await loadTrash();
    } catch (err) {
      showError(err);
    }
  }

  async function showNotesView() {
    setView("notes");
    state.activeTrash = null;
    renderList();
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
      await api("/api/notes", {
        method: "POST",
        body: JSON.stringify({ name, content: "" }),
      });
      setView("notes");
      await loadNotes();
      state.active = { name, content: "" };
      renderList();
    } catch (err) {
      showError(err);
    } finally {
      els.addBtn.disabled = false;
    }
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

  els.notesCard.addEventListener("click", goNotes);
  els.homeBtn.addEventListener("click", goHome);
  window.addEventListener("popstate", () => {
    applyScreen(location.hash === "#notes" ? "notes" : "home");
  });

  els.addBtn.addEventListener("click", () => createNote());
  els.trashBtn.addEventListener("click", () => {
    if (state.view === "trash") showNotesView();
    else showTrashView();
  });
  els.modalAction.addEventListener("click", () => {
    if (state.activeTrash) restoreActiveTrash();
    else trashActiveNote();
  });

  els.chatForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const text = els.chatInput.value.trim();
    if (!text || state.sending) return;
    els.chatInput.value = "";
    sendMessage(text);
  });

  els.chatInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      els.chatForm.requestSubmit();
    }
  });

  els.modalClose.addEventListener("click", closeNoteModal);

  function showError(err) {
    const item = document.createElement("li");
    item.className = "sidebar__empty sidebar__empty--error";
    item.textContent = err.message;
    els.list.prepend(item);
  }

  renderChat();
  if (location.hash === "#notes") applyScreen("notes");
})();
