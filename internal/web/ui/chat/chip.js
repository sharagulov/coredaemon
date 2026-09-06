import { el, icon } from "../dom.js";

export function fileLabel(path) {
  return String(path || "").replace(/^.*\//, "") || path;
}

export function folderLabel(path) {
  const parts = String(path || "").split("/").filter(Boolean);
  return parts[parts.length - 1] || path;
}

export function noteFolder(name) {
  const parts = String(name || "").split("/");
  if (parts.length <= 1) return "";
  return parts.slice(0, -1).join("/");
}

/**
 * @param {{ kind: string, path: string, label: string }} item
 * @param {{ closable?: boolean, onRemove?: (item: object) => void, onOpen?: (item: object) => void }} [opts]
 */
export function createChatChip(item, { closable = false, onRemove, onOpen } = {}) {
  const node = el(onOpen ? "button" : "span", "notes-chat__chip");
  if (onOpen) {
    node.type = "button";
    node.addEventListener("click", () => onOpen(item));
  }
  node.appendChild(icon(
    item.kind === "folder" ? "assets/icon-folder.png" : "assets/icon-file.png",
    "notes-chat__chip-icon",
  ));
  const label = el("span", "notes-chat__chip-label");
  label.textContent = item.label;
  node.appendChild(label);
  if (closable) {
    const close = el("button", "notes-chat__chip-close", { type: "button", "aria-label": "Убрать" });
    close.appendChild(icon("assets/icon-chat-close.png", "notes-chat__chip-x"));
    close.addEventListener("click", (e) => {
      e.stopPropagation();
      onRemove?.(item);
    });
    node.appendChild(close);
  }
  return node;
}
