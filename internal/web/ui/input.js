import { createIconButton } from "./button.js";

/**
 * @param {HTMLInputElement} input
 * @param {{ debounceMs?: number, onSearch: (value: string) => void, onClear?: () => void }} opts
 */
export function bindSearchInput(input, { debounceMs = 250, onSearch, onClear }) {
  let timer = 0;

  input.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(() => onSearch(input.value), debounceMs);
  });

  input.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      input.value = "";
      onClear?.();
      input.blur();
    }
  });

  return {
    clear() {
      input.value = "";
    },
  };
}

/**
 * @param {{
 *   formClass: string,
 *   inputClass: string,
 *   cancelClass: string,
 *   confirmClass: string,
 *   placeholder: string,
 *   maxLength?: number,
 *   onConfirm: (value: string) => void,
 *   onCancel: () => void,
 * }} opts
 */
export function createInlineForm({
  formClass,
  inputClass,
  cancelClass,
  confirmClass,
  placeholder,
  maxLength = 64,
  onConfirm,
  onCancel,
}) {
  const form = document.createElement("div");
  form.className = formClass;

  const input = document.createElement("input");
  input.className = inputClass;
  input.type = "text";
  input.placeholder = placeholder;
  input.maxLength = maxLength;
  input.autocomplete = "off";

  const submit = () => {
    const value = input.value.trim();
    if (value) onConfirm(value);
  };

  const cancel = createIconButton({
    className: cancelClass,
    ariaLabel: "Отмена",
    iconSrc: "assets/icon-close.svg",
    onClick: onCancel,
  });

  const confirm = createIconButton({
    className: confirmClass,
    ariaLabel: "Создать",
    iconSrc: "assets/icon-check.svg",
    onClick: submit,
  });

  input.addEventListener("keydown", (e) => {
    e.stopPropagation();
    if (e.key === "Enter") {
      e.preventDefault();
      submit();
    }
    if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  });

  form.append(input, cancel, confirm);
  requestAnimationFrame(() => input.focus());

  return form;
}
