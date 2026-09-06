import { el } from "../dom.js";
import { fileLabel } from "./chip.js";

/**
 * @param {string} name
 * @param {{ onOpen?: (name: string) => void }} [opts]
 */
export function createChatPill(name, { onOpen } = {}) {
  const node = el(onOpen ? "button" : "span", "notes-chat__pill");
  if (onOpen) {
    node.type = "button";
    node.addEventListener("click", () => onOpen(name));
  }
  node.textContent = fileLabel(name);
  return node;
}

/** @param {number} extra */
export function createChatMorePill(extra) {
  const node = el("span", "notes-chat__pill");
  node.textContent = `Ещё +${extra}`;
  return node;
}
