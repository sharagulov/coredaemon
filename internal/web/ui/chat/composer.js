import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { createMenuOption } from "../menu-option.js";
import { createChatChip, fileLabel, folderLabel, noteFolder } from "./chip.js";

function resizeTextarea(textarea) {
  textarea.style.height = "auto";
  textarea.style.height = `${Math.min(textarea.scrollHeight, 160)}px`;
}

/**
 * @param {{
 *   listNotes: () => { name: string, title?: string }[],
 *   noteLabel: (name: string) => string,
 *   onSubmit: (text: string, attachments: { kind: string, path: string, label: string }[]) => void,
 * }} opts
 */
export function createChatComposer({ listNotes, noteLabel, onSubmit }) {
  const form = el("form", "notes-chat__composer", { "aria-label": "Сообщение помощнику" });
  const box = el("div", "notes-chat__input");
  const textarea = el("textarea", "notes-chat__textarea", {
    name: "message",
    rows: "1",
    placeholder: "Начните общение с агентом",
  });

  const bar = el("div", "notes-chat__input-bar");
  const left = el("div", "notes-chat__input-left");
  const attachWrap = el("div", "notes-chat__attach-wrap");

  const attachBtn = el("button", "notes-chat__attach", {
    type: "button",
    "aria-label": "Прикрепить заметку",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
  });
  attachBtn.appendChild(icon("assets/icon-plus.png", "notes-chat__icon-slot notes-chat__icon-slot--plus"));

  const attachMenu = el("div", "notes-chat__attach-menu", {
    role: "listbox",
    "aria-label": "Заметки",
  });
  attachMenu.hidden = true;
  attachWrap.append(attachBtn, attachMenu);

  const chipsEl = el("div", "notes-chat__chips");
  left.append(attachWrap, chipsEl);

  const sendBtn = el("button", "notes-chat__send", { type: "submit", "aria-label": "Отправить" });
  sendBtn.appendChild(icon("assets/icon-chevron.png", "notes-chat__icon-slot notes-chat__icon-slot--send"));

  bar.append(left, sendBtn);
  box.append(textarea, bar);
  form.appendChild(box);

  let attachments = [];

  function renderChips() {
    chipsEl.innerHTML = "";
    for (const item of attachments) {
      chipsEl.appendChild(createChatChip(item, {
        closable: true,
        onRemove: (chip) => {
          attachments = attachments.filter((a) => !(a.kind === chip.kind && a.path === chip.path));
          renderChips();
        },
      }));
    }
  }

  function renderAttachMenu() {
    attachMenu.innerHTML = "";
    const notes = listNotes() || [];
    const taken = new Set(attachments.map((a) => `${a.kind}:${a.path}`));
    const folders = new Set();
    for (const note of notes) {
      const folder = noteFolder(note.name);
      if (folder) folders.add(folder);
    }

    for (const folder of [...folders].sort()) {
      const key = `folder:${folder}`;
      if (taken.has(key)) continue;
      attachMenu.appendChild(createMenuOption({
        className: "notes-chat__option",
        id: key,
        label: folderLabel(folder),
        dataKey: "value",
      }));
    }
    for (const note of notes) {
      const key = `file:${note.name}`;
      if (taken.has(key)) continue;
      attachMenu.appendChild(createMenuOption({
        className: "notes-chat__option",
        id: key,
        label: note.title || noteLabel(note.name),
        dataKey: "value",
      }));
    }
    if (!attachMenu.children.length) {
      const empty = el("div", "notes-chat__option notes-chat__option--empty");
      empty.textContent = "Нечего прикрепить";
      attachMenu.appendChild(empty);
    }
  }

  const dropdown = createDropdown({
    trigger: attachBtn,
    menu: attachMenu,
    optionSelector: ".notes-chat__option[data-value]",
    dataKey: "value",
    onSelect: (value) => {
      const i = value.indexOf(":");
      const kind = value.slice(0, i);
      const path = value.slice(i + 1);
      if (!kind || !path) return;
      if (attachments.some((a) => a.kind === kind && a.path === path)) return;
      attachments.push({
        kind,
        path,
        label: kind === "folder" ? folderLabel(path) : fileLabel(path),
      });
      renderChips();
      dropdown.close();
    },
    onOpen: renderAttachMenu,
    rootSelector: ".notes-chat__attach-wrap",
  });

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    const text = textarea.value.trim();
    if (!text) return;
    textarea.value = "";
    resizeTextarea(textarea);
    const items = attachments.map((a) => ({ ...a }));
    attachments = [];
    renderChips();
    onSubmit(text, items);
  });

  textarea.addEventListener("input", () => resizeTextarea(textarea));
  textarea.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      form.requestSubmit();
    }
  });

  return {
    el: form,
    rootSelector: ".notes-chat__attach-wrap",
    setBusy(busy) {
      textarea.disabled = busy;
      sendBtn.disabled = busy;
      attachBtn.disabled = busy;
    },
    closeMenu: dropdown.close,
    focus() {
      textarea.focus();
    },
  };
}
