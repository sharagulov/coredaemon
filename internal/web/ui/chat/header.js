import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { createMenuOption, syncMenuOptions } from "../menu-option.js";
import { createChatIconButton } from "./icon-btn.js";

/**
 * @param {{
 *   getChats: () => { id: string, title: string }[],
 *   getActiveId: () => string,
 *   onSelect: (id: string) => void,
 *   onNew: () => void,
 * }} opts
 */
export function createChatHeader({ getChats, getActiveId, onSelect, onNew }) {
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
  wrap.append(titleBtn, titleMenu);

  const newBtn = createChatIconButton({
    extraClass: "notes-chat__new",
    ariaLabel: "Новый чат",
    iconSrc: "assets/icon-plus.png",
    onClick: onNew,
  });

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
      titleMenu.innerHTML = "";
      const activeId = getActiveId();
      for (const chat of getChats()) {
        titleMenu.appendChild(createMenuOption({
          className: "notes-chat__option",
          id: chat.id,
          label: chat.title,
          active: chat.id === activeId,
          dataKey: "id",
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
    closeMenu: dropdown.close,
  };
}
