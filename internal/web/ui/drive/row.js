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
 * @param {{ selected?: boolean, pathHint?: string, onOpen: (entry: object) => void, onContextMenu?: (e: MouseEvent, entry: object) => void }} opts
 */
export function createDriveRow(entry, { selected = false, pathHint = "", onOpen, onContextMenu }) {
  const btn = el("button", "drive-tile", { type: "button" });
  btn.classList.toggle("is-selected", selected);
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
  btn.addEventListener("click", () => onOpen(entry));
  if (onContextMenu) {
    btn.addEventListener("contextmenu", (e) => {
      e.preventDefault();
      e.stopPropagation();
      onContextMenu(e, entry);
    });
  }
  return btn;
}
