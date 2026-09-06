import { el, icon } from "../dom.js";

export function secondsLabel(n) {
  const n10 = n % 10;
  const n100 = n % 100;
  if (n10 === 1 && n100 !== 11) return `${n} секунду`;
  if (n10 >= 2 && n10 <= 4 && (n100 < 12 || n100 > 14)) return `${n} секунды`;
  return `${n} секунд`;
}

/**
 * @param {"think" | "done"} kind
 * @param {string} text
 */
export function createChatStatus(kind, text) {
  const row = el("div", "notes-chat__status");
  row.appendChild(icon(
    kind === "think" ? "assets/icon-spinner.png" : "assets/icon-clock.png",
    kind === "think"
      ? "notes-chat__status-icon notes-chat__status-icon--spin"
      : "notes-chat__status-icon notes-chat__status-icon--clock",
  ));
  const label = el("span", "notes-chat__status-text");
  label.textContent = text;
  row.appendChild(label);
  return row;
}
