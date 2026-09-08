import { el } from "../dom.js";
import { createChatChip } from "./chip.js";
import { createChatIconButton } from "./icon-btn.js";
import { parseSlashInput, resolveCommand } from "./commands.js";

/**
 * @param {{
 *   content: string,
 *   attachments?: { kind: string, path: string, label: string }[],
 *   onRewind?: () => void,
 * }} opts
 */
export function createUserMessage({ content, attachments, onRewind }) {
  const card = el("div", "notes-chat__user");
  const text = el("p", "notes-chat__user-text");
  const parsed = parseSlashInput(content);
  if (parsed && resolveCommand(parsed)) {
    const cmd = el("code", "notes-chat__cmd");
    cmd.textContent = `/${parsed.tokenRaw}`;
    text.appendChild(cmd);
    if (parsed.rest) text.appendChild(document.createTextNode(` ${parsed.rest}`));
  } else {
    text.textContent = content;
  }
  card.appendChild(text);

  if (attachments?.length) {
    const row = el("div", "notes-chat__user-chips");
    for (const item of attachments) {
      row.appendChild(createChatChip(item));
    }
    card.appendChild(row);
  }

  card.appendChild(createChatIconButton({
    ariaLabel: "Откатить",
    iconSrc: "assets/icon-return.png",
    extraClass: "notes-chat__icon-btn--on-user",
    onClick: onRewind,
  }));
  return card;
}
