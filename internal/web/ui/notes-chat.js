import { el } from "./dom.js";
import {
  createBotError,
  createBotMessage,
  createBotPending,
  createChatComposer,
  createChatHeader,
  createChatModelPicker,
  createChatScopePicker,
  createSystemMessage,
  createUserMessage,
  expandSlash,
} from "./chat/index.js";

function isSystemMessage(msg) {
  return !!msg?.system;
}

const CHATS_KEY = "notes-chats-v1";
const OPEN_KEY = "notes-chat-open";
const PROVIDER_KEY = "notes-chat-provider";
const MAX_CHATS = 20;

function newId() {
  return crypto.randomUUID ? crypto.randomUUID() : `c-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function emptyChat() {
  return { id: newId(), title: "Новый чат", scope: "", messages: [] };
}

function emptyStore() {
  const chat = emptyChat();
  return { chats: [chat], activeId: chat.id };
}

function normalizeStore(raw) {
  if (!raw?.chats?.length) return null;
  const chats = raw.chats.filter((c) => c && c.id);
  if (!chats.length) return null;
  return {
    chats: chats.map((c) => ({ ...c, scope: c.scope || "" })),
    activeId: raw.activeId && chats.some((c) => c.id === raw.activeId)
      ? raw.activeId
      : chats[0].id,
  };
}

function loadLocalStore() {
  try {
    const loaded = normalizeStore(JSON.parse(localStorage.getItem(CHATS_KEY) || ""));
    if (loaded) {
      localStorage.removeItem(CHATS_KEY);
      return loaded;
    }
  } catch {
  }
  return emptyStore();
}

function titleFromText(text) {
  const t = String(text || "").replace(/\s+/g, " ").trim();
  if (!t) return "Новый чат";
  return t.length > 32 ? `${t.slice(0, 32).trim()}…` : t;
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

function isAbort(err) {
  return err?.name === "AbortError" || err?.message === "The user aborted a request.";
}

async function readChatStream(res, onEvent) {
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body.error) message = body.error;
    } catch {
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
      if (evt.event === "status") onEvent(evt.data);
      else if (evt.event === "done") return evt.data;
      else if (evt.event === "error") throw new Error(evt.data.error || "не удалось получить ответ");
    }
  }

  throw new Error("stream ended without result");
}

async function chatStream(messages, scope, attachments, provider, onEvent, signal) {
  const res = await fetch("/api/chat", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    },
    cache: "no-store",
    signal,
    body: JSON.stringify({
      messages,
      scope: scope || "",
      attachments: attachments || [],
      provider: provider || "ollama",
    }),
  });
  return readChatStream(res, onEvent);
}

function emptyUndo() {
  return { created: [], previous: {} };
}

function mergeUndo(into, src) {
  if (!src) return into;
  for (const file of src.created || []) {
    if (file && !into.created.includes(file)) into.created.push(file);
  }
  for (const [file, body] of Object.entries(src.previous || {})) {
    if (!file || into.created.includes(file) || Object.hasOwn(into.previous, file)) continue;
    into.previous[file] = body;
  }
  return into;
}

function undoFromMessages(messages) {
  const undo = emptyUndo();
  for (const msg of messages) mergeUndo(undo, msg);
  return undo;
}

function applyPhaseUndo(undo, phase) {
  if (!phase?.file) return;
  if (phase.kind === "created") {
    if (!undo.created.includes(phase.file)) undo.created.push(phase.file);
    return;
  }
  if (phase.kind !== "updated" || undo.created.includes(phase.file) || Object.hasOwn(undo.previous, phase.file)) {
    return;
  }
  if (Object.hasOwn(phase, "previous")) undo.previous[phase.file] = phase.previous;
}

function undoIsEmpty(undo) {
  return !undo.created.length && !Object.keys(undo.previous).length;
}

async function revertNotes(undo) {
  const res = await fetch("/api/rewind", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
    body: JSON.stringify({ created: undo.created, previous: undo.previous }),
  });
  if (res.ok) return;
  let message = res.statusText;
  try {
    const body = await res.json();
    if (body.error) message = body.error;
  } catch {
  }
  throw new Error(message);
}

export function createNotesChat({
  root,
  toggle,
  body,
  listNotes,
  noteLabel,
  getScopeTargets,
  noteMatchesScope,
  onNoteEvent,
  onNotesReload,
  onOpenNote,
}) {
  if (!root) return { closeMenus() {}, isOpen() { return false; }, attachNote() {} };

  const store = emptyStore();
  let ready = false;
  let sending = false;
  let pending = false;
  let pendingSince = 0;
  let abort = null;
  let sendGate = Promise.resolve();
  let turnUndo = emptyUndo();
  let providers = [{ id: "ollama", label: "Локальная", available: true }];
  let provider = localStorage.getItem(PROVIDER_KEY) || "ollama";

  function activeChat() {
    return store.chats.find((c) => c.id === store.activeId) || store.chats[0];
  }

  function persist() {
    return fetch("/api/chats", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      cache: "no-store",
      body: JSON.stringify({ chats: store.chats, activeId: store.activeId }),
    }).catch(() => {});
  }

  async function loadProviders() {
    try {
      const res = await fetch("/api/models", { cache: "no-store" });
      if (!res.ok) return;
      const data = await res.json();
      if (Array.isArray(data.providers) && data.providers.length) {
        providers = data.providers;
      }
    } catch {
    }
    if (!providers.some((p) => p.id === provider && p.available)) {
      provider = providers.find((p) => p.available)?.id || "ollama";
    }
    modelPicker.sync();
  }

  async function hydrate() {
    let loaded = null;
    try {
      const res = await fetch("/api/chats", { cache: "no-store" });
      if (res.ok) loaded = normalizeStore(await res.json());
    } catch {
    }
    await loadProviders();
    if (!loaded) {
      loaded = loadLocalStore();
      store.chats = loaded.chats;
      store.activeId = loaded.activeId;
      ready = true;
      render();
      await persist();
      return;
    }
    store.chats = loaded.chats;
    store.activeId = loaded.activeId;
    ready = true;
    render();
  }

  const messagesEl = el("div", "notes-chat__messages");

  let requestClose = () => {};

  const header = createChatHeader({
    getChats: () => store.chats,
    getActiveId: () => store.activeId,
    onSelect: switchChat,
    onNew: startNewChat,
    onDelete: (id) => { deleteChat(id).catch(() => {}); },
    onClose: () => requestClose(),
  });

  function scopedNotes() {
    const all = listNotes() || [];
    const scope = activeChat()?.scope || "";
    if (!scope || !noteMatchesScope) return all;
    return all.filter((n) => noteMatchesScope(n, scope));
  }

  function activeScopeLabel() {
    const id = activeChat()?.scope || "";
    if (!id) return "";
    return getScopeTargets().find((t) => t.id === id)?.label || "";
  }

  function syncScopeChip() {
    composer.setScopeChip(activeScopeLabel());
  }

  function setChatScope(id) {
    if (!ready || sending) return;
    const chat = activeChat();
    const next = id || "";
    if ((chat.scope || "") === next) return;
    chat.scope = next;
    persist();
    scopePicker.sync();
    syncScopeChip();
  }

  let closeAttachMenu = () => {};

  const scopePicker = createChatScopePicker({
    getTargets: getScopeTargets,
    getActiveId: () => activeChat()?.scope || "",
    onSelect: setChatScope,
    onOpen: () => {
      closeAttachMenu();
      modelPicker.close();
    },
  });

  function setProvider(id) {
    if (!providers.some((p) => p.id === id && p.available)) return;
    provider = id;
    localStorage.setItem(PROVIDER_KEY, id);
    modelPicker.sync();
  }

  const modelPicker = createChatModelPicker({
    getProviders: () => providers,
    getActiveId: () => provider,
    onSelect: setProvider,
    onOpen: () => {
      closeAttachMenu();
      scopePicker.close();
    },
  });

  const composer = createChatComposer({
    listNotes: scopedNotes,
    noteLabel,
    scopePicker: scopePicker.el,
    modelPicker,
    onClearScope: () => setChatScope(""),
    onSubmit: (text, attachments) => sendMessage(text, attachments),
    onStop: stopGenerating,
  });
  closeAttachMenu = composer.closeMenu;

  root.replaceChildren(header.el, messagesEl, composer.el);

  function setOpen(open) {
    body.classList.toggle("is-chat-open", open);
    if (toggle) {
      toggle.classList.toggle("is-collapsed", !open);
      toggle.setAttribute("aria-pressed", open ? "true" : "false");
      const compact = window.matchMedia("(max-width: 1100px)").matches;
      toggle.setAttribute(
        "aria-label",
        open
          ? (compact ? "Закрыть чат" : "Свернуть чат")
          : (compact ? "Открыть чат" : "Развернуть чат"),
      );
    }
    localStorage.setItem(OPEN_KEY, open ? "1" : "0");
    body.dispatchEvent(new CustomEvent("notes-chat-toggle"));
  }

  requestClose = () => setOpen(false);

  function isOpen() {
    return body.classList.contains("is-chat-open");
  }

  function setBusy(busy) {
    header.setBusy(busy);
  }

  function stopGenerating() {
    abort?.abort();
  }

  function closeMenus() {
    header.closeMenu();
    composer.closeMenu();
    scopePicker.close();
    modelPicker.close();
  }

  function attachNote(name) {
    if (!name) return;
    composer.attach({ kind: "file", path: name });
    setOpen(true);
    composer.focus();
  }

  // Facts the daemon reported go back as system messages: given them in the assistant
  // voice the model copies the shape and invents reports like "Удалено: Мышки.md".
  function history() {
    return activeChat().messages
      .filter((m) => m.role === "user" || m.role === "assistant")
      .map((m) => ({
        role: isSystemMessage(m) ? "system" : m.role,
        content: m.role === "user" ? expandSlash(m.content) : m.content,
      }));
  }

  function render() {
    messagesEl.innerHTML = "";
    const chat = activeChat();
    let turn = null;

    const flush = () => {
      if (turn) messagesEl.appendChild(turn);
      turn = null;
    };

    chat.messages.forEach((msg, index) => {
      if (msg.role === "user") {
        flush();
        turn = el("div", "notes-chat__turn");
        turn.appendChild(createUserMessage({
          content: msg.content,
          attachments: msg.attachments,
          onRewind: () => rewind(index),
        }));
        return;
      }
      if (!turn) turn = el("div", "notes-chat__turn");
      if (msg.role === "assistant") {
        if (isSystemMessage(msg)) {
          turn.appendChild(createSystemMessage({
            content: msg.content,
            at: msg.at,
            created: msg.created,
            updated: msg.updated,
            onOpenNote,
          }));
        } else {
          turn.appendChild(createBotMessage({
            content: msg.content,
            thoughtSec: msg.thoughtSec,
            at: msg.at,
            created: msg.created,
            updated: msg.updated,
            searched: msg.searched,
            matches: msg.matches,
            onOpenNote,
          }));
        }
      }
      if (msg.role === "error") {
        turn.appendChild(createBotError(msg.content));
      }
    });

    if (pending) {
      if (!turn) turn = el("div", "notes-chat__turn");
      turn.appendChild(createBotPending());
    }

    flush();
    messagesEl.scrollTop = messagesEl.scrollHeight;
    header.setTitle(activeChat().title);
    scopePicker.sync();
    syncScopeChip();
  }

  function switchChat(id) {
    if (!ready || sending || !store.chats.some((c) => c.id === id)) return;
    store.activeId = id;
    persist();
    render();
  }

  function startNewChat() {
    if (!ready || sending) return;
    const current = activeChat();
    if (!current.messages.length) return;
    const chat = emptyChat();
    store.chats.unshift(chat);
    store.activeId = chat.id;
    if (store.chats.length > MAX_CHATS) store.chats.pop();
    persist();
    render();
  }

  async function deleteChat(id) {
    if (!ready) return;
    const idx = store.chats.findIndex((c) => c.id === id);
    if (idx === -1) return;

    if (sending && store.activeId === id) {
      stopGenerating();
      await sendGate;
    }

    store.chats.splice(idx, 1);
    if (!store.chats.length) {
      const chat = emptyChat();
      store.chats = [chat];
      store.activeId = chat.id;
    } else if (store.activeId === id) {
      store.activeId = store.chats[Math.min(idx, store.chats.length - 1)].id;
    }

    header.closeMenu();
    await persist();
    render();
  }

  async function rewind(index) {
    if (!ready) return;
    const chat = activeChat();
    const msg = chat.messages[index];
    if (!msg || msg.role !== "user") return;

    if (sending) {
      stopGenerating();
      await sendGate;
    }
    if (chat.messages[index] !== msg) return;

    const undo = mergeUndo(undoFromMessages(chat.messages.slice(index)), turnUndo);
    turnUndo = emptyUndo();

    if (!undoIsEmpty(undo)) {
      try {
        await revertNotes(undo);
      } catch (err) {
        chat.messages.push({ role: "error", content: err.message });
        persist();
        render();
        return;
      }
      try {
        await onNotesReload?.();
      } catch {
      }
    }

    chat.messages = chat.messages.slice(0, index);
    persist();
    render();
    composer.setDraft(msg.content, msg.attachments || []);
    composer.focus();
  }

  async function sendMessage(text, attached = []) {
    if (!ready || sending || !text.trim()) return;

    const chat = activeChat();
    let releaseGate = () => {};
    sendGate = new Promise((resolve) => { releaseGate = resolve; });
    abort = new AbortController();
    sending = true;
    pending = true;
    pendingSince = Date.now();
    turnUndo = emptyUndo();
    header.setBusy(true);
    composer.setGenerating(true);

    const userMsg = {
      role: "user",
      content: text,
      attachments: attached.map((a) => ({ ...a })),
    };
    chat.messages.push(userMsg);
    if (chat.title === "Новый чат") chat.title = titleFromText(text);
    await persist();
    render();

    try {
      const res = await chatStream(
        history(),
        chat.scope || "",
        attached.map((a) => a.path).filter(Boolean),
        provider,
        (phase) => {
          applyPhaseUndo(turnUndo, phase);
          onNoteEvent?.(phase);
        },
        abort.signal,
      );

      if (!chat.messages.includes(userMsg)) return;

      chat.messages.push({
        role: "assistant",
        content: res.content,
        at: Date.now(),
        system: !!res.system,
        created: res.created || turnUndo.created,
        updated: res.updated || [],
        previous: res.previous || turnUndo.previous,
        searched: !!res.searched,
        matches: res.matches || [],
        thoughtSec: Math.max(1, Math.round((Date.now() - pendingSince) / 1000)),
      });

      if (res.notes_changed) await onNotesReload?.();
    } catch (err) {
      if (!chat.messages.includes(userMsg)) return;
      if (isAbort(err)) {
        if (!undoIsEmpty(turnUndo)) {
          userMsg.created = turnUndo.created;
          userMsg.previous = turnUndo.previous;
        }
      } else {
        chat.messages.push({ role: "error", content: err.message });
      }
    } finally {
      abort = null;
      sending = false;
      pending = false;
      header.setBusy(false);
      composer.setGenerating(false);
      releaseGate();
      if (chat.messages.includes(userMsg)) {
        await persist();
        render();
        composer.focus();
      }
    }
  }

  if (toggle) {
    toggle.addEventListener("click", () => setOpen(!isOpen()));
  }

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape" || !isOpen()) return;
    if (!window.matchMedia("(max-width: 1100px)").matches) return;
    if (e.target.closest("textarea, input, [contenteditable='true']")) return;
    setOpen(false);
  });

  const savedOpen = localStorage.getItem(OPEN_KEY);
  const compact = window.matchMedia("(max-width: 1100px)").matches;
  const defaultOpen = compact ? false : true;
  setOpen(savedOpen == null ? defaultOpen : savedOpen !== "0");
  render();
  setBusy(true);
  hydrate().finally(() => {
    if (!sending) setBusy(false);
  });

  return { closeMenus, closeContextMenu: header.closeContextMenu, isOpen, setOpen, attachNote };
}
