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
  let placed = false;

  function placeMenu() {
    if (!menu || !trigger || menu.hidden) return;
    const rect = trigger.getBoundingClientRect();
    const gap = 6;
    const pad = 8;

    menu.style.position = "fixed";
    menu.style.right = "auto";
    menu.style.bottom = "auto";
    menu.style.zIndex = "400";
    menu.style.top = `${Math.round(rect.bottom + gap)}px`;
    menu.style.left = `${Math.round(rect.left)}px`;

    const mw = menu.offsetWidth;
    const mh = menu.offsetHeight;
    let left = rect.right - mw;
    if (left < pad) left = rect.left;
    if (left + mw > window.innerWidth - pad) {
      left = Math.max(pad, window.innerWidth - pad - mw);
    }

    let top = rect.bottom + gap;
    if (top + mh > window.innerHeight - pad && rect.top - gap - mh >= pad) {
      top = rect.top - gap - mh;
    }

    menu.style.left = `${Math.round(left)}px`;
    menu.style.top = `${Math.round(top)}px`;
  }

  function bindPlace() {
    if (placed) return;
    placed = true;
    window.addEventListener("resize", placeMenu);
    document.addEventListener("scroll", placeMenu, true);
  }

  function unbindPlace() {
    if (!placed) return;
    placed = false;
    window.removeEventListener("resize", placeMenu);
    document.removeEventListener("scroll", placeMenu, true);
  }

  function clearPlace() {
    if (!menu) return;
    menu.style.position = "";
    menu.style.top = "";
    menu.style.left = "";
    menu.style.right = "";
    menu.style.bottom = "";
    menu.style.zIndex = "";
  }

  function close() {
    if (!menu || !trigger) return;
    menu.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
    trigger.classList.remove("is-open");
    unbindPlace();
    clearPlace();
    onClose?.();
  }

  function open() {
    if (!menu || !trigger) return;
    onOpen?.();
    menu.hidden = false;
    trigger.setAttribute("aria-expanded", "true");
    trigger.classList.add("is-open");
    bindPlace();
    placeMenu();
    requestAnimationFrame(placeMenu);
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
    e.stopPropagation();
    const opt = e.target.closest(optionSelector);
    if (!opt) return;
    onSelect(opt.dataset[dataKey]);
  });

  return { open, close, toggle, rootSelector };
}
