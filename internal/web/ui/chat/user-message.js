import { el } from "../dom.js";
import { createChatChip } from "./chip.js";
import { createChatIconButton } from "./icon-btn.js";

/**
 * @param {{
 *   content: string,
 *   attachments?: { kind: string, path: string, label: string }[],
 *   onRegenerate?: () => void,
 * }} opts
 */
export function createUserMessage({ content, attachments, onRegenerate }) {
  const card = el("div", "notes-chat__user");
  const text = el("p", "notes-chat__user-text");
  text.textContent = content;
  card.appendChild(text);

  if (attachments?.length) {
    const row = el("div", "notes-chat__user-chips");
    for (const item of attachments) {
      row.appendChild(createChatChip(item));
    }
    card.appendChild(row);
  }

  card.appendChild(createChatIconButton({
    ariaLabel: "Повторить",
    iconSrc: "assets/icon-return.png",
    extraClass: "notes-chat__icon-btn--on-user",
    onClick: onRegenerate,
  }));
  return card;
}
