import { el } from "./dom.js";
import {
  createBotError,
  createBotMessage,
  createBotPending,
  createChatComposer,
  createChatHeader,
  createUserMessage,
} from "./chat/index.js";

const CHATS_KEY = "notes-chats-v1";
const OPEN_KEY = "notes-chat-open";
const MAX_CHATS = 20;

function newId() {
  return crypto.randomUUID ? crypto.randomUUID() : `c-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function emptyChat() {
  return { id: newId(), title: "Новый чат", messages: [] };
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
    chats,
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
      else if (evt.event === "error") throw new Error(evt.data.error || "ollama unavailable");
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

function contextSuffix(items) {
  if (!items?.length) return "";
  return `\n\nКонтекст: ${items.map((a) => a.path).join(", ")}`;
}

export function createNotesChat({
  root,
  toggle,
  body,
  listNotes,
  noteLabel,
  onNoteEvent,
  onNotesReload,
  onOpenNote,
}) {
  if (!root) return { closeMenus() {}, isOpen() { return false; } };

  const store = emptyStore();
  let ready = false;
  let sending = false;
  let pending = false;
  let pendingSince = 0;

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

  async function hydrate() {
    let loaded = null;
    try {
      const res = await fetch("/api/chats", { cache: "no-store" });
      if (res.ok) loaded = normalizeStore(await res.json());
    } catch {
    }
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

  const header = createChatHeader({
    getChats: () => store.chats,
    getActiveId: () => store.activeId,
    onSelect: switchChat,
    onNew: startNewChat,
  });

  const composer = createChatComposer({
    listNotes,
    noteLabel,
    onSubmit: (text, attachments) => sendMessage(text, attachments),
  });

  root.replaceChildren(header.el, messagesEl, composer.el);

  function setOpen(open) {
    body.classList.toggle("is-chat-open", open);
    if (toggle) {
      toggle.classList.toggle("is-collapsed", !open);
      toggle.setAttribute("aria-pressed", open ? "true" : "false");
      toggle.setAttribute("aria-label", open ? "Свернуть чат" : "Развернуть чат");
    }
    localStorage.setItem(OPEN_KEY, open ? "1" : "0");
    body.dispatchEvent(new CustomEvent("notes-chat-toggle"));
  }

  function isOpen() {
    return body.classList.contains("is-chat-open");
  }

  function setBusy(busy) {
    header.setBusy(busy);
    composer.setBusy(busy);
  }

  function closeMenus() {
    header.closeMenu();
    composer.closeMenu();
  }

  function history() {
    return activeChat().messages
      .filter((m) => m.role === "user" || m.role === "assistant")
      .map((m) => ({
        role: m.role,
        content: m.role === "user"
          ? `${m.content}${contextSuffix(m.attachments)}`
          : m.content,
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
          onRegenerate: () => regenerate(index),
        }));
        return;
      }
      if (!turn) turn = el("div", "notes-chat__turn");
      if (msg.role === "assistant") {
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

  async function regenerate(index) {
    if (!ready || sending) return;
    const chat = activeChat();
    const msg = chat.messages[index];
    if (!msg || msg.role !== "user") return;
    chat.messages = chat.messages.slice(0, index);
    persist();
    await sendMessage(msg.content, msg.attachments || []);
  }

  async function sendMessage(text, attached = []) {
    if (!ready || sending || !text.trim()) return;

    const chat = activeChat();
    sending = true;
    pending = true;
    pendingSince = Date.now();
    setBusy(true);

    chat.messages.push({
      role: "user",
      content: text,
      attachments: attached.map((a) => ({ ...a })),
    });
    if (chat.title === "Новый чат") chat.title = titleFromText(text);
    await persist();
    render();

    try {
      const res = await chatStream(history(), (phase) => {
        onNoteEvent?.(phase);
      });

      chat.messages.push({
        role: "assistant",
        content: res.content,
        at: Date.now(),
        created: res.created || [],
        updated: res.updated || [],
        searched: !!res.searched,
        matches: res.matches || [],
        thoughtSec: Math.max(1, Math.round((Date.now() - pendingSince) / 1000)),
      });

      if (res.notes_changed) await onNotesReload?.();
    } catch (err) {
      chat.messages.push({ role: "error", content: err.message });
    } finally {
      sending = false;
      pending = false;
      setBusy(false);
      await persist();
      render();
      composer.focus();
    }
  }

  if (toggle) {
    toggle.addEventListener("click", () => setOpen(!isOpen()));
  }

  const savedOpen = localStorage.getItem(OPEN_KEY);
  setOpen(savedOpen == null ? true : savedOpen !== "0");
  render();
  setBusy(true);
  hydrate().finally(() => {
    if (!sending) setBusy(false);
  });

  return { closeMenus, isOpen, setOpen };
}
