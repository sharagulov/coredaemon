import { mountMarkdownCM } from "../cm-bundle.js";

const isMac = /Mac|iPhone|iPad/.test(navigator.platform || "");
const mod = isMac ? "⌘" : "Ctrl";

function shortcut(...keys) {
  return [mod, ...keys].join(" + ");
}

const TOOLBAR = [
  { id: "bold", label: "Жирный", icon: "assets/editor-bold.svg", shortcut: shortcut("B") },
  { id: "italic", label: "Курсив", icon: "assets/editor-italic.svg", shortcut: shortcut("I") },
  { id: "underline", label: "Подчёркнутый", icon: "assets/editor-underline.svg", shortcut: shortcut("U") },
  { id: "strike", label: "Зачёркнутый", icon: "assets/editor-strikethrough.svg", shortcut: shortcut("Shift", "X") },
  { id: "link", label: "Ссылка", icon: "assets/editor-link.svg", shortcut: shortcut("K") },
  { id: "code", label: "Код", icon: "assets/editor-code.svg", shortcut: shortcut("E") },
  { id: "heading", label: "Заголовок", icon: "assets/editor-type.svg", shortcut: shortcut("Shift", "H") },
  { id: "list", label: "Список", icon: "assets/editor-list.svg", shortcut: shortcut("Shift", "L") },
  { id: "quote", label: "Цитата", icon: "assets/editor-quote.svg", shortcut: shortcut("Shift", ".") },
];

const HISTORY = [
  {
    id: "undo",
    label: "Отменить",
    icon: "assets/editor-undo.png",
    shortcut: shortcut("Z"),
    keys: "Control+Z Meta+Z",
  },
  {
    id: "redo",
    label: "Повторить",
    icon: "assets/editor-redo.png",
    shortcut: isMac ? shortcut("Shift", "Z") : shortcut("Y"),
    keys: "Control+Y Meta+Shift+Z",
  },
];

function createToolButton({ label, iconSrc, shortcut: keys, onClick }) {
  const wrap = document.createElement("span");
  wrap.className = "notes-editor__tool-wrap";
  if (keys) wrap.dataset.shortcut = keys;

  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "notes-editor__tool";
  btn.setAttribute("aria-label", label);
  if (keys) btn.setAttribute("aria-keyshortcuts", keys.replaceAll(" + ", "+"));
  const slot = document.createElement("span");
  slot.className = "notes-editor__tool-icon";
  const img = document.createElement("img");
  img.src = iconSrc;
  img.alt = "";
  slot.appendChild(img);
  btn.appendChild(slot);
  btn.addEventListener("mousedown", (e) => e.preventDefault());
  btn.addEventListener("click", (e) => {
    e.preventDefault();
    onClick();
  });
  wrap.appendChild(btn);
  return { wrap, btn };
}

export function createMarkdownEditor({ panel, onChange }) {
  const toolbar = panel.querySelector(".notes-editor__toolbar");
  const mount = panel.querySelector(".notes-editor__cm") || panel.querySelector(".notes-editor__area");
  const historyBtns = {};
  const cm = mountMarkdownCM(mount, {
    onChange,
    onHistory: (flags) => {
      if (historyBtns.undo) historyBtns.undo.disabled = !flags.undo;
      if (historyBtns.redo) historyBtns.redo.disabled = !flags.redo;
    },
  });

  for (const item of TOOLBAR) {
    const { wrap } = createToolButton({
      label: item.label,
      iconSrc: item.icon,
      shortcut: item.shortcut,
      onClick: () => cm.run(item.id),
    });
    toolbar.appendChild(wrap);
  }

  const historyGroup = document.createElement("div");
  historyGroup.className = "notes-editor__history";
  for (const item of HISTORY) {
    const { wrap, btn } = createToolButton({
      label: item.label,
      iconSrc: item.icon,
      shortcut: item.shortcut,
      onClick: () => cm.run(item.id),
    });
    btn.setAttribute("aria-keyshortcuts", item.keys);
    btn.disabled = true;
    historyBtns[item.id] = btn;
    historyGroup.appendChild(wrap);
  }
  toolbar.appendChild(historyGroup);

  return {
    setContent(text) {
      cm.setContent(text || "");
    },
    getContent() {
      return cm.getContent();
    },
    setReadOnly(on) {
      toolbar.hidden = on;
      cm.setReadOnly(on);
    },
    focus() {
      cm.focus();
    },
  };
}
