import { el } from "./dom.js";

/**
 * @returns {{
 *   el: HTMLElement,
 *   open: (opts: {
 *     title: string,
 *     message: string,
 *     confirmLabel?: string,
 *     onConfirm: () => void,
 *     onCancel?: () => void,
 *   }) => void,
 *   close: () => void,
 * }}
 */
export function createConfirmModal() {
  const root = el("div", "ui-modal", {
    hidden: "",
    "aria-hidden": "true",
  });
  const dialog = el("div", "ui-modal__dialog", {
    role: "alertdialog",
    "aria-modal": "true",
    tabindex: "-1",
  });
  const titleEl = el("h2", "ui-modal__title");
  const messageEl = el("p", "ui-modal__message");
  const actions = el("div", "ui-modal__actions");
  const cancelBtn = el("button", "ui-modal__btn", { type: "button" });
  cancelBtn.textContent = "Отмена";
  const okBtn = el("button", "ui-modal__btn ui-modal__btn--danger", { type: "button" });
  okBtn.textContent = "Удалить";
  actions.append(cancelBtn, okBtn);
  dialog.append(titleEl, messageEl, actions);
  root.appendChild(dialog);

  let onConfirm = null;
  let onCancel = null;
  let prevFocus = null;

  function onKey(e) {
    if (e.key !== "Escape") return;
    e.preventDefault();
    e.stopPropagation();
    finish(false);
  }

  function finish(ok) {
    if (root.hidden) return;
    root.hidden = true;
    root.setAttribute("aria-hidden", "true");
    document.removeEventListener("keydown", onKey, true);
    const confirm = onConfirm;
    const cancel = onCancel;
    const restore = prevFocus;
    onConfirm = null;
    onCancel = null;
    prevFocus = null;
    if (ok) confirm?.();
    else {
      cancel?.();
      if (restore && document.contains(restore)) restore.focus();
    }
  }

  cancelBtn.addEventListener("click", () => finish(false));
  okBtn.addEventListener("click", () => finish(true));
  root.addEventListener("click", (e) => {
    if (e.target === root) finish(false);
  });
  dialog.addEventListener("keydown", (e) => {
    if (e.key !== "Tab") return;
    e.preventDefault();
    if (e.shiftKey) {
      (document.activeElement === cancelBtn ? okBtn : cancelBtn).focus();
    } else {
      (document.activeElement === okBtn ? cancelBtn : okBtn).focus();
    }
  });

  function open({ title, message, confirmLabel = "Удалить", onConfirm: nextConfirm, onCancel: nextCancel }) {
    titleEl.textContent = title;
    messageEl.textContent = message;
    okBtn.textContent = confirmLabel;
    dialog.setAttribute("aria-label", `${title} ${message}`);
    onConfirm = nextConfirm;
    onCancel = nextCancel || null;
    prevFocus = document.activeElement;
    root.hidden = false;
    root.setAttribute("aria-hidden", "false");
    document.addEventListener("keydown", onKey, true);
    cancelBtn.focus();
  }

  return {
    el: root,
    open,
    close: () => finish(false),
  };
}
