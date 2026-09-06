import { el, icon } from "./dom.js";
import { createDropdown } from "./dropdown.js";

/**
 * @param {{ id: string, label: string, active: boolean }} opts
 */
function createSectionOption({ id, label, active }) {
  const btn = el("button", "notes-filter__option notes-editor__section-option", {
    type: "button",
    role: "option",
    "aria-selected": active ? "true" : "false",
  });
  btn.dataset.section = id;

  const text = el("span", "notes-editor__section-option-label");
  text.textContent = label;

  const mark = icon("assets/icon-check.svg", "notes-editor__section-check");
  mark.hidden = !active;

  btn.append(text, mark);
  return btn;
}

/**
 * @param {{
 *   getLabel: () => string,
 *   getTargets: () => { id: string, label: string }[],
 *   getActiveId: () => string,
 *   isDisabled: () => boolean,
 *   onSelect: (id: string) => void,
 *   onOpen?: () => void,
 * }} opts
 */
export function createNoteSectionPicker({
  getLabel,
  getTargets,
  getActiveId,
  isDisabled,
  onSelect,
  onOpen,
}) {
  const root = el("div", "notes-editor__section");
  const btn = el("button", "notes-bar notes-bar--dropdown notes-editor__section-btn", {
    type: "button",
    "aria-label": "Раздел заметки",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
  });
  const labelEl = el("span", "notes-editor__section-label");
  btn.append(
    labelEl,
    icon("assets/icon-chevron.png", "notes-bar__icon-slot notes-bar__icon-slot--chevron"),
  );

  const menu = el("div", "notes-filter__menu notes-editor__section-menu", {
    role: "listbox",
    "aria-label": "Разделы заметки",
  });
  menu.hidden = true;
  root.append(btn, menu);

  const dropdown = createDropdown({
    trigger: btn,
    menu,
    optionSelector: ".notes-editor__section-option",
    dataKey: "section",
    rootSelector: ".notes-editor__section",
    onOpen: () => {
      onOpen?.();
      menu.innerHTML = "";
      const activeId = getActiveId();
      for (const target of getTargets()) {
        menu.appendChild(createSectionOption({
          id: target.id,
          label: target.label,
          active: target.id === activeId,
        }));
      }
    },
    onSelect: (id) => {
      dropdown.close();
      onSelect(id);
    },
  });

  function sync() {
    labelEl.textContent = getLabel();
    btn.disabled = isDisabled();
    if (btn.disabled) dropdown.close();
  }

  return { el: root, sync, close: dropdown.close };
}
