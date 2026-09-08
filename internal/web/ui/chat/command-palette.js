import { el } from "../dom.js";
import { commandPreview, matchCommands, parseSlashInput } from "./commands.js";

/**
 * @param {{
 *   getAnchor?: () => Element | null,
 *   onPick: (cmd: object) => void,
 * }} opts
 */
export function createCommandPalette({ getAnchor, onPick }) {
  const root = el("div", "notes-chat__commands", {
    id: "notes-chat-commands",
    hidden: "",
    role: "listbox",
    "aria-label": "Команды",
  });

  let items = [];
  let index = 0;
  let parsed = null;

  function hide() {
    root.hidden = true;
    items = [];
    index = 0;
    parsed = null;
  }

  function render() {
    root.replaceChildren();
    items.forEach((cmd, i) => {
      const btn = el("button", i === index ? "notes-chat__command is-active" : "notes-chat__command", {
        type: "button",
        role: "option",
        id: `chat-cmd-${cmd.id}`,
        "aria-selected": i === index ? "true" : "false",
      });
      const slash = el("span", "notes-chat__command-slash");
      slash.textContent = `/${cmd.names[0]}`;
      const meta = el("span", "notes-chat__command-meta");
      const title = el("span", "notes-chat__command-title");
      title.textContent = cmd.title;
      meta.appendChild(title);
      const extra = commandPreview(cmd, parsed?.rest);
      if (extra) {
        const hint = el("span", "notes-chat__command-hint");
        hint.textContent = extra;
        meta.appendChild(hint);
      }
      btn.append(slash, meta);
      btn.addEventListener("mousedown", (e) => e.preventDefault());
      btn.addEventListener("click", () => onPick(cmd));
      root.appendChild(btn);
    });
    root.children[index]?.scrollIntoView({ block: "nearest" });
  }

  function sync(text) {
    parsed = parseSlashInput(text);
    if (!parsed) {
      hide();
      return false;
    }
    items = matchCommands(parsed);
    if (!items.length) {
      hide();
      return false;
    }
    if (index >= items.length) index = items.length - 1;
    if (index < 0) index = 0;
    root.hidden = false;
    render();
    return true;
  }

  function move(delta) {
    if (!items.length) return;
    index = (index + delta + items.length) % items.length;
    render();
  }

  document.addEventListener("pointerdown", (e) => {
    if (root.hidden) return;
    if (root.contains(e.target)) return;
    if (getAnchor?.()?.contains(e.target)) return;
    hide();
  });

  return {
    el: root,
    isOpen: () => !root.hidden,
    sync,
    hide,
    next: () => move(1),
    prev: () => move(-1),
    selected: () => items[index] || null,
    activeId: () => (items[index] ? `chat-cmd-${items[index].id}` : ""),
  };
}
