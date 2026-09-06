import { el } from "../dom.js";
import { renderMarkdown } from "../markdown.js";
import { createChatIconButton } from "./icon-btn.js";
import { createChatMorePill, createChatPill } from "./pill.js";
import { createChatStatus, secondsLabel } from "./status.js";
import { createChatTime } from "./time.js";

const BADGE_LIMIT = 2;

function createPills(files, limit, onOpenNote) {
  const shown = files.slice(0, limit);
  if (!shown.length) return null;
  const pills = el("div", "notes-chat__pills");
  for (const file of shown) {
    pills.appendChild(createChatPill(file, { onOpen: onOpenNote }));
  }
  if (files.length > limit) {
    pills.appendChild(createChatMorePill(files.length - limit));
  }
  return pills;
}

/**
 * @param {{
 *   content?: string,
 *   thoughtSec?: number,
 *   at?: number|string|Date,
 *   created?: string[],
 *   updated?: string[],
 *   searched?: boolean,
 *   matches?: { file?: string }[],
 *   onOpenNote?: (name: string) => void,
 * }} opts
 */
export function createBotMessage({
  content,
  thoughtSec,
  at,
  created,
  updated,
  searched,
  matches,
  onOpenNote,
}) {
  const wrap = el("div", "notes-chat__bot");
  if (thoughtSec > 0) {
    wrap.appendChild(createChatStatus("done", `Думал ${secondsLabel(thoughtSec)}`));
  }

  const md = el("div", "notes-chat__md md-preview");
  renderMarkdown(md, content || "", { copyIcon: "assets/icon-chat-copy.png", chat: true });
  wrap.appendChild(md);

  const actions = el("div", "notes-chat__actions");
  const meta = el("div", "notes-chat__meta");
  meta.appendChild(createChatIconButton({
    ariaLabel: "Копировать",
    iconSrc: "assets/icon-chat-copy.png",
    onClick: () => {
      navigator.clipboard.writeText(content || "").catch(() => {});
    },
  }));
  const time = createChatTime(at);
  if (time) meta.appendChild(time);
  actions.appendChild(meta);

  const written = [...new Set([...(created || []), ...(updated || [])].filter(Boolean))];
  const found = [];
  if (searched) {
    for (const hit of matches || []) {
      if (hit.file && !written.includes(hit.file)) found.push(hit.file);
    }
  }
  const files = [...written, ...found];
  const pills = createPills(files, written.length ? 8 : BADGE_LIMIT, onOpenNote);
  if (pills) actions.appendChild(pills);

  wrap.appendChild(actions);
  return wrap;
}

export function createBotPending() {
  const wrap = el("div", "notes-chat__bot");
  wrap.appendChild(createChatStatus("think", "Думаю"));
  return wrap;
}

/**
 * @param {{
 *   content?: string,
 *   at?: number|string|Date,
 *   created?: string[],
 *   updated?: string[],
 *   onOpenNote?: (name: string) => void,
 * }} opts
 */
export function createSystemMessage({ content, at, created, updated, onOpenNote }) {
  const wrap = el("div", "notes-chat__system");
  const badge = el("span", "notes-chat__system-badge");
  badge.textContent = "Системное";
  wrap.appendChild(badge);
  const text = el("p", "notes-chat__system-text");
  text.textContent = content || "";
  wrap.appendChild(text);
  const files = [...new Set([...(created || []), ...(updated || [])].filter(Boolean))];
  const pills = createPills(files, 8, onOpenNote);
  if (pills) wrap.appendChild(pills);
  const time = createChatTime(at);
  if (time) wrap.appendChild(time);
  return wrap;
}

/** @param {string} text */
export function createBotError(text) {
  const err = el("p", "notes-chat__error");
  err.textContent = text;
  return err;
}
