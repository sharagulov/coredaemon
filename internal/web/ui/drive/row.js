import { el, icon } from "../dom.js";

export function driveParentPath(rel) {
  const i = String(rel || "").lastIndexOf("/");
  return i < 0 ? "" : rel.slice(0, i);
}

/** @param {string} path */
export function driveFileUrl(path) {
  return "/api/drive/file/" + String(path).split("/").map(encodeURIComponent).join("/");
}

/**
 * @param {{
 *   path: string,
 *   name: string,
 *   is_dir?: boolean,
 *   kind?: string,
 * }} entry
 * @param {{
 *   selected?: boolean,
 *   focused?: boolean,
 *   pathHint?: string,
 *   onSelect: (e: MouseEvent, entry: object) => void,
 *   onActivate: (entry: object) => void,
 *   onContextMenu?: (e: MouseEvent, entry: object) => void,
 * }} opts
 */
export function createDriveRow(entry, { selected = false, focused = false, pathHint = "", onSelect, onActivate, onContextMenu }) {
  const btn = el("button", "drive-tile", { type: "button" });
  btn.dataset.path = entry.path;
  btn.tabIndex = focused ? 0 : -1;
  btn.classList.toggle("is-selected", selected);
  btn.classList.toggle("is-focused", focused);
  const kind = entry.is_dir ? "folder" : entry.kind || "other";
  const src = kind === "folder" ? "assets/icon-folder.png" : "assets/icon-file.png";
  btn.appendChild(icon(src, "drive-tile__icon"));
  const name = el("span", "drive-tile__name");
  name.textContent = entry.name;
  name.title = entry.path || entry.name;
  btn.appendChild(name);
  if (pathHint) {
    const sub = el("span", "drive-tile__path");
    sub.textContent = pathHint;
    sub.title = pathHint;
    btn.appendChild(sub);
  }

  let longPress = false;
  let pressTimer = null;

  btn.addEventListener("click", (e) => {
    e.stopPropagation();
    if (longPress) {
      longPress = false;
      return;
    }
    if (e.detail > 1) return;
    onSelect(e, entry);
  });
  btn.addEventListener("dblclick", (e) => {
    e.preventDefault();
    e.stopPropagation();
    onActivate(entry);
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
        onContextMenu({ clientX, clientY, preventDefault() {} }, entry);
      }, 500);
    });
    btn.addEventListener("pointerup", cancelPress);
    btn.addEventListener("pointercancel", cancelPress);
    btn.addEventListener("pointerleave", cancelPress);
    btn.addEventListener("contextmenu", (e) => {
      e.preventDefault();
      e.stopPropagation();
      onContextMenu(e, entry);
    });
  }
  return btn;
}
