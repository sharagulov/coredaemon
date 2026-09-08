import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { syncMenuOptions } from "../menu-option.js";
import { createContextMenu, createContextAction } from "../context-menu.js";
import { createChatIconButton } from "./icon-btn.js";
import { createChatOption } from "./chat-option.js";

/**
 * @param {{
 *   getChats: () => { id: string, title: string }[],
 *   getActiveId: () => string,
 *   onSelect: (id: string) => void,
 *   onNew: () => void,
 *   onDelete: (id: string) => void,
 *   onClose?: () => void,
 * }} opts
 */
export function createChatHeader({ getChats, getActiveId, onSelect, onNew, onDelete, onClose }) {
  const header = el("header", "notes-chat__header");
  const wrap = el("div", "notes-chat__title-wrap");

  const titleBtn = el("button", "notes-chat__title-btn", {
    type: "button",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
  });
  const titleEl = el("span", "notes-chat__title");
  titleEl.textContent = "Новый чат";
  const chevron = icon("assets/icon-chevron.png", "notes-chat__title-chevron");
  chevron.setAttribute("aria-hidden", "true");
  titleBtn.append(titleEl, chevron);

  const titleMenu = el("div", "notes-chat__title-menu", {
    role: "listbox",
    "aria-label": "Чаты",
  });
  titleMenu.hidden = true;

  const ctxRoot = el("div", "notes-chat__ctx", { hidden: "", "aria-hidden": "true" });
  const ctxMenu = el("div", "notes-ctx__menu", { role: "menu" });
  ctxRoot.appendChild(ctxMenu);

  wrap.append(titleBtn, titleMenu, ctxRoot);

  const chatCtx = createContextMenu({ root: ctxRoot, menu: ctxMenu });
  chatCtx.mount();

  function openChatContextMenu(e, id) {
    chatCtx.openAt(e, (menu) => {
      menu.appendChild(createContextAction({
        label: "Удалить",
        danger: true,
        onClick: () => {
          chatCtx.close();
          onDelete(id);
        },
      }));
    });
  }

  const newBtn = createChatIconButton({
    extraClass: "notes-chat__new",
    ariaLabel: "Новый чат",
    iconSrc: "assets/icon-plus.png",
    onClick: onNew,
  });

  if (onClose) {
    const closeBtn = createChatIconButton({
      extraClass: "notes-chat__close",
      ariaLabel: "Закрыть чат",
      iconSrc: "assets/icon-close.svg",
      onClick: onClose,
    });
    header.append(closeBtn);
  }

  header.append(wrap, newBtn);

  const dropdown = createDropdown({
    trigger: titleBtn,
    menu: titleMenu,
    optionSelector: ".notes-chat__option",
    dataKey: "id",
    onSelect: (id) => {
      onSelect(id);
      dropdown.close();
    },
    onOpen: () => {
      chatCtx.close();
      titleMenu.innerHTML = "";
      const activeId = getActiveId();
      for (const chat of getChats()) {
        titleMenu.appendChild(createChatOption({
          id: chat.id,
          label: chat.title,
          active: chat.id === activeId,
          onContextMenu: openChatContextMenu,
        }));
      }
      syncMenuOptions(titleMenu, ".notes-chat__option", activeId, "id");
    },
    rootSelector: ".notes-chat__title-wrap",
  });

  return {
    el: header,
    rootSelector: ".notes-chat__title-wrap",
    setTitle(title) {
      titleEl.textContent = title;
    },
    setBusy(busy) {
      newBtn.disabled = busy;
    },
    closeMenu() {
      chatCtx.close();
      dropdown.close();
    },
    closeContextMenu: chatCtx.close,
  };
}
