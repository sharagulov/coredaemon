import { el } from "../dom.js";
import { renderMarkdown } from "../markdown.js";
import { createChatIconButton } from "./icon-btn.js";
import { createChatMorePill, createChatPill } from "./pill.js";
import { createChatStatus, secondsLabel } from "./status.js";
import { createChatTime } from "./time.js";

const BADGE_LIMIT = 2;

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
  renderMarkdown(md, content || "", { copyIcon: "assets/icon-chat-copy.png" });
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

  const files = [...(created || []), ...(updated || [])];
  if (searched) {
    for (const hit of matches || []) {
      if (hit.file) files.push(hit.file);
    }
  }
  const unique = [...new Set(files)];
  if (unique.length) {
    const pills = el("div", "notes-chat__pills");
    for (const file of unique.slice(0, BADGE_LIMIT)) {
      pills.appendChild(createChatPill(file, { onOpen: onOpenNote }));
    }
    if (unique.length > BADGE_LIMIT) {
      pills.appendChild(createChatMorePill(unique.length - BADGE_LIMIT));
    }
    actions.appendChild(pills);
  }

  wrap.appendChild(actions);
  return wrap;
}

export function createBotPending() {
  const wrap = el("div", "notes-chat__bot");
  wrap.appendChild(createChatStatus("think", "Думаю"));
  return wrap;
}

/**
 * @param {{ content?: string, at?: number|string|Date }} opts
 */
export function createSystemMessage({ content, at }) {
  const wrap = el("div", "notes-chat__system");
  const badge = el("span", "notes-chat__system-badge");
  badge.textContent = "Системное";
  wrap.appendChild(badge);
  const text = el("p", "notes-chat__system-text");
  text.textContent = content || "";
  wrap.appendChild(text);
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
