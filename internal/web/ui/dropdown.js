/**
 * @param {{
 *   trigger: Element,
 *   menu: Element,
 *   optionSelector: string,
 *   dataKey: string,
 *   onSelect: (value: string) => void,
 *   onOpen?: () => void,
 *   onClose?: () => void,
 *   rootSelector: string,
 * }} opts
 */
export function createDropdown({
  trigger,
  menu,
  optionSelector,
  dataKey,
  onSelect,
  onOpen,
  onClose,
  rootSelector,
}) {
  function close() {
    if (!menu || !trigger) return;
    menu.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
    trigger.classList.remove("is-open");
    onClose?.();
  }

  function open() {
    if (!menu || !trigger) return;
    onOpen?.();
    menu.hidden = false;
    trigger.setAttribute("aria-expanded", "true");
    trigger.classList.add("is-open");
  }

  function toggle() {
    if (menu.hidden) open();
    else close();
  }

  trigger?.addEventListener("click", (e) => {
    e.stopPropagation();
    toggle();
  });

  menu?.addEventListener("click", (e) => {
    const opt = e.target.closest(optionSelector);
    if (!opt) return;
    onSelect(opt.dataset[dataKey]);
  });

  return { open, close, toggle, rootSelector };
}
