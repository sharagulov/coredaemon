import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { createMenuOption, syncMenuOptions } from "../menu-option.js";

/**
 * @param {{
 *   getProviders: () => { id: string, label: string, model?: string, available?: boolean }[],
 *   getActiveId: () => string,
 *   onSelect: (id: string) => void,
 *   onOpen?: () => void,
 * }} opts
 */
export function createChatModelPicker({ getProviders, getActiveId, onSelect, onOpen }) {
  const wrap = el("div", "notes-chat__model-wrap");
  const btn = el("button", "notes-chat__model", {
    type: "button",
    "aria-label": "Модель",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
  });
  const label = el("span", "notes-chat__model-label");
  const chevron = icon("assets/icon-chevron.png", "notes-chat__icon-slot notes-chat__model-chevron");
  chevron.setAttribute("aria-hidden", "true");
  btn.append(label, chevron);

  const menu = el("div", "notes-chat__attach-menu notes-chat__model-menu", {
    role: "listbox",
    "aria-label": "Модель",
  });
  menu.hidden = true;
  wrap.append(btn, menu);

  const dropdown = createDropdown({
    trigger: btn,
    menu,
    optionSelector: ".notes-chat__model-option:not(:disabled)",
    dataKey: "provider",
    rootSelector: ".notes-chat__model-wrap",
    onOpen: () => {
      onOpen?.();
      renderMenu();
    },
    onSelect: (id) => {
      dropdown.close();
      onSelect(id);
    },
  });

  function activeProvider() {
    const id = getActiveId();
    return (getProviders() || []).find((p) => p.id === id) || getProviders()?.[0];
  }

  function renderMenu() {
    menu.innerHTML = "";
    const activeId = getActiveId();
    for (const provider of getProviders() || []) {
      const opt = createMenuOption({
        className: "notes-chat__option notes-chat__model-option",
        id: provider.id,
        label: optionLabel(provider),
        active: provider.id === activeId,
        dataKey: "provider",
      });
      if (!provider.available) {
        opt.disabled = true;
        opt.textContent = `${provider.label} — нет ключа`;
      }
      menu.appendChild(opt);
    }
    syncMenuOptions(menu, ".notes-chat__model-option", activeId, "provider");
  }

  function optionLabel(provider) {
    if (provider.model) return `${provider.label} · ${provider.model}`;
    return provider.label;
  }

  function sync() {
    const current = activeProvider();
    label.textContent = current?.label || "Локальная";
    const title = current?.model ? `${current.label} (${current.model})` : (current?.label || "Модель");
    btn.setAttribute("aria-label", `Модель: ${title}`);
    btn.title = title;
  }

  sync();

  return {
    el: wrap,
    sync,
    close: dropdown.close,
    rootSelector: ".notes-chat__model-wrap",
    setDisabled(on) {
      btn.disabled = !!on;
    },
  };
}