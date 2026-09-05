/**
 * @param {{ className: string, id: string, label: string, active?: boolean, dataKey?: string }} opts
 */
export function createMenuOption({ className, id, label, active = false, dataKey = "value" }) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = className;
  btn.role = "option";
  btn.dataset[dataKey] = id;
  btn.textContent = label;
  btn.classList.toggle("is-active", active);
  btn.setAttribute("aria-selected", active ? "true" : "false");
  return btn;
}

/** @param {Element} menu @param {string} selector @param {string} value @param {string} dataKey */
export function syncMenuOptions(menu, selector, value, dataKey) {
  for (const btn of menu.querySelectorAll(selector)) {
    const active = btn.dataset[dataKey] === value;
    btn.classList.toggle("is-active", active);
    btn.setAttribute("aria-selected", active ? "true" : "false");
  }
}
