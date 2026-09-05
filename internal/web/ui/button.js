import { icon } from "./dom.js";

/**
 * @param {{ className: string, label: string, iconSrc?: string, iconClass?: string, onClick: () => void }} opts
 */
export function createAddMenuButton({ className, label, iconSrc, iconClass, onClick }) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = className;
  if (iconSrc) btn.appendChild(icon(iconSrc, iconClass));
  const text = document.createElement("span");
  text.textContent = label;
  btn.appendChild(text);
  btn.addEventListener("click", (e) => {
    e.stopPropagation();
    onClick();
  });
  return btn;
}

/**
 * @param {{ className: string, label: string, ariaLabel?: string, onClick: () => void }} opts
 */
export function createMenuButton({ className, label, ariaLabel, onClick }) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = className;
  btn.setAttribute("role", "menuitem");
  if (ariaLabel) btn.setAttribute("aria-label", ariaLabel);
  btn.textContent = label;
  btn.addEventListener("click", (e) => {
    e.stopPropagation();
    onClick();
  });
  return btn;
}

/**
 * @param {{ className: string, ariaLabel: string, iconSrc: string, confirm?: boolean, onClick: () => void }} opts
 */
export function createIconButton({ className, ariaLabel, iconSrc, confirm, onClick }) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = className;
  if (confirm) btn.classList.add(`${className}--confirm`);
  btn.setAttribute("aria-label", ariaLabel);
  btn.innerHTML = `<img src="${iconSrc}" alt="">`;
  btn.addEventListener("click", (e) => {
    e.stopPropagation();
    onClick();
  });
  return btn;
}
