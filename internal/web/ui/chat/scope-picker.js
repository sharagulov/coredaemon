import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { createMenuOption, syncMenuOptions } from "../menu-option.js";

/**
 * @param {{
 *   getTargets: () => { id: string, label: string }[],
 *   getActiveId: () => string,
 *   onSelect: (id: string) => void,
 *   onOpen?: () => void,
 * }} opts
 */
export function createChatScopePicker({ getTargets, getActiveId, onSelect, onOpen }) {
  const wrap = el("div", "notes-chat__scope-wrap");
  const btn = el("button", "notes-chat__scope", {
    type: "button",
    "aria-label": "Раздел для чата",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
  });
  btn.appendChild(icon("assets/icon-folder.png", "notes-chat__icon-slot notes-chat__icon-slot--scope"));

  const menu = el("div", "notes-chat__attach-menu notes-chat__scope-menu", {
    role: "listbox",
    "aria-label": "Раздел для чата",
  });
  menu.hidden = true;
  wrap.append(btn, menu);

  const dropdown = createDropdown({
    trigger: btn,
    menu,
    optionSelector: ".notes-chat__scope-option",
    dataKey: "scope",
    rootSelector: ".notes-chat__scope-wrap",
    onOpen: () => {
      onOpen?.();
      menu.innerHTML = "";
      const activeId = getActiveId();
      for (const target of getTargets()) {
        menu.appendChild(createMenuOption({
          className: "notes-chat__option notes-chat__scope-option",
          id: target.id,
          label: target.label,
          active: target.id === activeId,
          dataKey: "scope",
        }));
      }
      syncMenuOptions(menu, ".notes-chat__scope-option", activeId, "scope");
    },
    onSelect: (id) => {
      dropdown.close();
      onSelect(id);
    },
  });

  function sync() {
    const active = !!getActiveId();
    btn.classList.toggle("is-active", active);
    btn.setAttribute("aria-label", active ? `Раздел: ${getTargets().find((t) => t.id === getActiveId())?.label || ""}` : "Раздел для чата");
  }

  return { el: wrap, sync, close: dropdown.close, rootSelector: ".notes-chat__scope-wrap" };
}
