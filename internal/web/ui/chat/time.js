import { el } from "../dom.js";

export function formatClock(at) {
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return "";
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

/** @param {number|string|Date} [at] */
export function createChatTime(at) {
  const clock = formatClock(at);
  if (!clock) return null;
  const time = el("time", "notes-chat__time", { datetime: new Date(at).toISOString() });
  time.textContent = clock;
  return time;
}
