import { el, icon } from "../dom.js";

/**
 * @param {{
 *   ariaLabel: string,
 *   iconSrc: string,
 *   iconClass?: string,
 *   extraClass?: string,
 *   onClick?: (e: MouseEvent) => void,
 * }} opts
 */
export function createChatIconButton({
  ariaLabel,
  iconSrc,
  iconClass = "notes-chat__icon-slot",
  extraClass = "",
  onClick,
}) {
  const btn = el("button", extraClass ? `notes-chat__icon-btn ${extraClass}` : "notes-chat__icon-btn", {
    type: "button",
    "aria-label": ariaLabel,
  });
  btn.appendChild(icon(iconSrc, iconClass));
  if (onClick) btn.addEventListener("click", onClick);
  return btn;
}
