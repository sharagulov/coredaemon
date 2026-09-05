import { icon } from "./dom.js";
import { createMenuOption } from "./menu-option.js";
import { createMenuButton } from "./button.js";

/**
 * @param {{ root: Element, menu: Element }} els
 */
export function createContextMenu({ root, menu }) {
  function close() {
    if (!root) return;
    root.hidden = true;
    root.setAttribute("aria-hidden", "true");
  }

  /**
   * @param {MouseEvent} e
   * @param {(menuEl: Element) => void} render
   */
  function openAt(e, render) {
    if (!root || !menu) return;
    menu.innerHTML = "";
    render(menu);
    root.hidden = false;
    root.setAttribute("aria-hidden", "false");

    const pad = 8;
    let x = e.clientX;
    let y = e.clientY;
    root.style.left = `${x}px`;
    root.style.top = `${y}px`;

    requestAnimationFrame(() => {
      const rect = root.getBoundingClientRect();
      if (x + rect.width > window.innerWidth - pad) {
        x = Math.max(pad, window.innerWidth - rect.width - pad);
      }
      if (y + rect.height > window.innerHeight - pad) {
        y = Math.max(pad, window.innerHeight - rect.height - pad);
      }
      root.style.left = `${x}px`;
      root.style.top = `${y}px`;
    });
  }

  function mount() {
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
  }

  return { close, openAt, mount, rootSelector: ".notes-ctx" };
}

/**
 * @param {{
 *   label: string,
 *   items: { id: string, label: string, active?: boolean }[],
 *   onSelect: (id: string) => void,
 * }} opts
 */
export function createSubmenuRow({ label, items, onSelect }) {
  const row = document.createElement("div");
  row.className = "notes-ctx__item notes-ctx__item--sub";
  row.setAttribute("role", "menuitem");

  const labelEl = document.createElement("span");
  labelEl.textContent = label;
  row.appendChild(labelEl);
  row.appendChild(icon("assets/icon-chevron.png", "notes-ctx__chevron"));

  const submenu = document.createElement("div");
  submenu.className = "notes-ctx__submenu";
  submenu.setAttribute("role", "menu");

  for (const item of items) {
    const btn = createMenuOption({
      className: "notes-ctx__option",
      id: item.id,
      label: item.label,
      active: item.active,
      dataKey: "value",
    });
    btn.setAttribute("role", "menuitem");
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      onSelect(item.id);
    });
    submenu.appendChild(btn);
  }

  row.appendChild(submenu);
  return row;
}

/** @param {{ label: string, danger?: boolean, onClick: () => void }} opts */
export function createContextAction({ label, danger, onClick }) {
  return createMenuButton({
    className: danger ? "notes-ctx__item notes-ctx__item--danger" : "notes-ctx__item",
    label,
    onClick,
  });
}
