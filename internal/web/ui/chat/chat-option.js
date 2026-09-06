import { el, icon } from "../dom.js";

/**
 * @param {{ id: string, label: string, active?: boolean, onDelete: (id: string) => void }} opts
 */
export function createChatOption({ id, label, active = false, onDelete }) {
  const row = el("div", "notes-chat__chat-row");

  const btn = el("button", "notes-chat__option", {
    type: "button",
    role: "option",
  });
  btn.dataset.id = id;
  btn.textContent = label;
  btn.classList.toggle("is-active", active);
  btn.setAttribute("aria-selected", active ? "true" : "false");

  const del = el("button", "notes-chat__chat-del", {
    type: "button",
    "aria-label": "Удалить чат",
  });
  del.appendChild(icon("assets/icon-chat-close.png", "notes-chat__chat-del-icon"));
  del.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    onDelete(id);
  });

  row.append(btn, del);
  return row;
}
