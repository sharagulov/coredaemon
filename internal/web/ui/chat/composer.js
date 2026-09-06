import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { createChatChip, fileLabel, folderLabel } from "./chip.js";
import { renderAttachMenu } from "./attach-menu.js";

function resizeTextarea(textarea) {
  textarea.style.height = "auto";
  textarea.style.height = `${Math.min(textarea.scrollHeight, 160)}px`;
}

/**
 * @param {{
 *   listNotes: () => { name: string, title?: string }[],
 *   noteLabel: (name: string) => string,
 *   onSubmit: (text: string, attachments: { kind: string, path: string, label: string }[]) => void,
 *   onStop?: () => void,
 * }} opts
 */
export function createChatComposer({ listNotes, noteLabel, onSubmit, onStop }) {
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
  const attachSearch = el("input", "notes-chat__attach-search", {
    type: "search",
    placeholder: "Поиск",
    autocomplete: "off",
    "aria-label": "Поиск заметок",
  });
  const attachList = el("div", "notes-chat__attach-list");
  attachMenu.hidden = true;
  attachMenu.append(attachSearch, attachList);
  attachWrap.append(attachBtn, attachMenu);

  const chipsEl = el("div", "notes-chat__chips");
  left.append(attachWrap, chipsEl);

  const sendIcon = icon("assets/icon-chevron.png", "notes-chat__icon-slot notes-chat__icon-slot--send");
  const stopIcon = icon("assets/icon-stop.svg", "notes-chat__icon-slot notes-chat__icon-slot--stop");
  const sendBtn = el("button", "notes-chat__send", { type: "submit", "aria-label": "Отправить" });
  sendBtn.appendChild(sendIcon);

  bar.append(left, sendBtn);
  box.append(textarea, bar);
  form.appendChild(box);

  let attachments = [];
  let generating = false;
  let attachExpanded = new Set();

  function clearAttachMenuPos() {
    attachMenu.style.left = "";
    attachMenu.style.maxWidth = "";
  }

  function fitAttachMenu() {
    if (attachMenu.hidden) return;
    attachMenu.style.left = "0px";
    const pad = 8;
    const chat = attachWrap.closest(".notes-chat");
    const box = chat?.getBoundingClientRect();
    const leftBound = Math.max(pad, (box?.left ?? 0) + pad);
    const rightBound = Math.min(window.innerWidth - pad, (box?.right ?? window.innerWidth) - pad);
    attachMenu.style.maxWidth = `${Math.max(0, Math.min(420, rightBound - leftBound))}px`;
    const wrap = attachWrap.getBoundingClientRect();
    const menu = attachMenu.getBoundingClientRect();
    let left = 0;
    if (menu.right > rightBound) left -= menu.right - rightBound;
    if (wrap.left + left < leftBound) left = leftBound - wrap.left;
    attachMenu.style.left = `${Math.round(left)}px`;
  }

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

  function attach(item) {
    const kind = String(item?.kind || "");
    const path = String(item?.path || "");
    if (!kind || !path) return;
    if (attachments.some((a) => a.kind === kind && a.path === path)) return;
    attachments.push({
      kind,
      path,
      label: item.label || (kind === "folder" ? folderLabel(path) : fileLabel(path)),
    });
    renderChips();
  }

  function renderAttachList() {
    renderAttachMenu(attachList, {
      notes: listNotes() || [],
      taken: new Set(attachments.map((a) => `${a.kind}:${a.path}`)),
      expanded: attachExpanded,
      noteLabel,
      query: attachSearch.value,
    });
  }

  attachSearch.addEventListener("click", (e) => e.stopPropagation());
  attachSearch.addEventListener("keydown", (e) => {
    if (e.key === "Enter") e.preventDefault();
    e.stopPropagation();
  });
  attachSearch.addEventListener("input", () => {
    attachExpanded.clear();
    renderAttachList();
    fitAttachMenu();
  });

  attachMenu.addEventListener("click", (e) => {
    const toggle = e.target.closest(".notes-chat__attach-toggle");
    if (!toggle) return;
    e.stopPropagation();
    const path = toggle.dataset.path;
    if (!path) return;
    if (attachExpanded.has(path)) attachExpanded.delete(path);
    else attachExpanded.add(path);
    renderAttachList();
    fitAttachMenu();
  });

  const dropdown = createDropdown({
    trigger: attachBtn,
    menu: attachMenu,
    optionSelector: ".notes-chat__attach-row[data-value]",
    dataKey: "value",
    onSelect: (value) => {
      const i = value.indexOf(":");
      attach({ kind: value.slice(0, i), path: value.slice(i + 1) });
      dropdown.close();
    },
    onOpen: () => {
      attachExpanded.clear();
      attachSearch.value = "";
      renderAttachList();
      queueMicrotask(() => {
        fitAttachMenu();
        attachSearch.focus();
      });
    },
    onClose: clearAttachMenuPos,
    rootSelector: ".notes-chat__attach-wrap",
  });

  window.addEventListener("resize", fitAttachMenu);
  queueMicrotask(() => {
    const chatEl = attachWrap.closest(".notes-chat");
    if (typeof ResizeObserver === "undefined" || !chatEl) return;
    new ResizeObserver(() => fitAttachMenu()).observe(chatEl);
  });

  sendBtn.addEventListener("click", (e) => {
    if (!generating) return;
    e.preventDefault();
    onStop?.();
  });

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    if (generating) return;
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
    setGenerating(on) {
      generating = !!on;
      sendBtn.type = generating ? "button" : "submit";
      sendBtn.setAttribute("aria-label", generating ? "Остановить" : "Отправить");
      sendBtn.classList.toggle("notes-chat__send--stop", generating);
      sendBtn.replaceChildren(generating ? stopIcon : sendIcon);
    },
    closeMenu: dropdown.close,
    attach,
    setDraft(text, items = []) {
      textarea.value = text || "";
      attachments = (items || []).map((a) => ({ ...a }));
      renderChips();
      resizeTextarea(textarea);
    },
    focus() {
      textarea.focus();
    },
  };
}
