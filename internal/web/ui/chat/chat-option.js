import { createMenuOption } from "../menu-option.js";

/**
 * @param {{
 *   id: string,
 *   label: string,
 *   active?: boolean,
 *   onContextMenu: (e: MouseEvent, id: string) => void,
 * }} opts
 */
export function createChatOption({ id, label, active = false, onContextMenu }) {
  const btn = createMenuOption({
    className: "notes-chat__option",
    id,
    label,
    active,
    dataKey: "id",
  });
  btn.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    e.stopPropagation();
    onContextMenu(e, id);
  });
  return btn;
}
