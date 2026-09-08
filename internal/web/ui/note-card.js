/**
 * @param {{
 *   path: string,
 *   title: string,
 *   preview: string,
 *   date: string,
 *   isActive?: boolean,
 *   onClick: () => void,
 *   onContextMenu?: (e: MouseEvent) => void,
 * }} opts
 */
export function createNoteCard({ path, title, preview, date, isActive, onClick, onContextMenu }) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "notes-card";
  if (isActive) btn.classList.add("is-active");
  btn.title = path;

  const body = document.createElement("div");
  body.className = "notes-card__body";

  const titleEl = document.createElement("span");
  titleEl.className = "notes-card__title";
  titleEl.textContent = title;
  body.appendChild(titleEl);

  const previewEl = document.createElement("span");
  previewEl.className = "notes-card__preview";
  previewEl.textContent = preview;
  body.appendChild(previewEl);

  btn.appendChild(body);

  const dateEl = document.createElement("span");
  dateEl.className = "notes-card__date";
  dateEl.textContent = date;
  btn.appendChild(dateEl);

  let longPress = false;
  let pressTimer = null;

  btn.addEventListener("click", (e) => {
    if (longPress) {
      longPress = false;
      return;
    }
    onClick();
  });
  if (onContextMenu) {
    const cancelPress = () => {
      if (pressTimer != null) {
        clearTimeout(pressTimer);
        pressTimer = null;
      }
    };
    btn.addEventListener("pointerdown", (e) => {
      if (e.pointerType !== "touch") return;
      longPress = false;
      cancelPress();
      const { clientX, clientY } = e;
      pressTimer = window.setTimeout(() => {
        pressTimer = null;
        longPress = true;
        onContextMenu({ clientX, clientY, preventDefault() {} });
      }, 500);
    });
    btn.addEventListener("pointerup", cancelPress);
    btn.addEventListener("pointercancel", cancelPress);
    btn.addEventListener("pointerleave", cancelPress);
    btn.addEventListener("contextmenu", (e) => {
      e.preventDefault();
      onContextMenu(e);
    });
  }

  return btn;
}
