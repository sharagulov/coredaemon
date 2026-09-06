import { el, icon } from "../dom.js";

/**
 * @param {{ name: string, title?: string }[]} notes
 */
function buildTree(notes) {
  const root = { dirs: new Map(), files: [] };
  for (let rank = 0; rank < notes.length; rank++) {
    const note = notes[rank];
    const parts = note.name.split("/");
    parts.pop();
    if (!parts.length) {
      root.files.push({ note, rank });
      continue;
    }
    let node = root;
    let path = "";
    for (const part of parts) {
      path = path ? `${path}/${part}` : part;
      if (!node.dirs.has(path)) {
        node.dirs.set(path, { path, segment: part, dirs: new Map(), files: [], rank });
      }
      const dir = node.dirs.get(path);
      if (dir.rank === undefined || rank < dir.rank) dir.rank = rank;
      node = dir;
    }
    node.files.push({ note, rank });
  }
  return root;
}

function sortedDirs(node) {
  return [...node.dirs.values()].sort((a, b) => (a.rank ?? 0) - (b.rank ?? 0));
}

function sortedFiles(node) {
  return [...node.files].sort((a, b) => (a.rank ?? 0) - (b.rank ?? 0));
}

function dirHasVisible(dir, taken) {
  if (taken.has(`folder:${dir.path}`)) return false;
  for (const f of dir.files) {
    if (!taken.has(`file:${f.note.name}`)) return true;
  }
  for (const sub of dir.dirs.values()) {
    if (dirHasVisible(sub, taken)) return true;
  }
  return false;
}

function dirExpandable(dir, taken) {
  for (const f of dir.files) {
    if (!taken.has(`file:${f.note.name}`)) return true;
  }
  for (const sub of dir.dirs.values()) {
    if (dirHasVisible(sub, taken)) return true;
  }
  return false;
}

/**
 * @param {{
 *   kind: "folder"|"file",
 *   value: string,
 *   label: string,
 *   path: string,
 *   depth: number,
 *   expandable?: boolean,
 *   expanded?: boolean,
 * }} row
 */
function createAttachItem({ kind, value, label, path, depth, expandable, expanded }) {
  const item = el("div", "notes-chat__attach-item");
  item.style.setProperty("--depth", String(depth));

  if (expandable) {
    const toggle = el("button", "notes-chat__attach-toggle", {
      type: "button",
      "aria-label": expanded ? "Свернуть" : "Раскрыть",
      "aria-expanded": expanded ? "true" : "false",
      "data-path": path,
    });
    toggle.classList.toggle("is-expanded", !!expanded);
    toggle.appendChild(icon("assets/icon-chevron.png", "notes-chat__attach-toggle-icon"));
    item.appendChild(toggle);
  }

  const row = el("button", "notes-chat__option notes-chat__attach-row", {
    type: "button",
    role: "option",
    "data-value": value,
    "aria-selected": "false",
  });

  row.appendChild(icon(
    kind === "folder" ? "assets/icon-folder.png" : "assets/icon-file.png",
    "notes-chat__attach-icon",
  ));

  const labelEl = el("span", "notes-chat__attach-label");
  labelEl.textContent = label;
  row.appendChild(labelEl);

  const pathEl = el("span", "notes-chat__attach-path");
  pathEl.textContent = path;
  pathEl.title = path;
  row.appendChild(pathEl);

  item.appendChild(row);
  return item;
}

function normalizeQuery(q) {
  return String(q || "").trim().toLowerCase();
}

function noteMatches(note, q, noteLabel) {
  if (!q) return true;
  const title = String(note.title || noteLabel(note.name) || "").toLowerCase();
  const name = String(note.name || "").toLowerCase();
  return title.includes(q) || name.includes(q);
}

function expandAncestors(notes, expanded) {
  for (const note of notes) {
    const parts = String(note.name || "").split("/");
    parts.pop();
    let path = "";
    for (const part of parts) {
      path = path ? `${path}/${part}` : part;
      expanded.add(path);
    }
  }
}

/**
 * @param {Element} menu
 * @param {{
 *   notes: { name: string, title?: string }[],
 *   taken: Set<string>,
 *   expanded: Set<string>,
 *   noteLabel: (name: string) => string,
 *   query?: string,
 * }} opts
 * @returns {number}
 */
export function renderAttachMenu(menu, { notes, taken, expanded, noteLabel, query = "" }) {
  menu.innerHTML = "";
  const q = normalizeQuery(query);
  const visible = q ? (notes || []).filter((n) => noteMatches(n, q, noteLabel)) : (notes || []);
  if (q) expandAncestors(visible, expanded);
  const root = buildTree(visible);
  let count = 0;

  function walk(node, depth) {
    for (const dir of sortedDirs(node)) {
      if (!dirHasVisible(dir, taken)) continue;
      menu.appendChild(createAttachItem({
        kind: "folder",
        value: `folder:${dir.path}`,
        label: dir.segment,
        path: dir.path,
        depth,
        expandable: dirExpandable(dir, taken),
        expanded: expanded.has(dir.path),
      }));
      count++;
      if (expanded.has(dir.path)) walk(dir, depth + 1);
    }
    for (const { note } of sortedFiles(node)) {
      const key = `file:${note.name}`;
      if (taken.has(key)) continue;
      menu.appendChild(createAttachItem({
        kind: "file",
        value: key,
        label: note.title || noteLabel(note.name),
        path: note.name,
        depth,
        expandable: false,
      }));
      count++;
    }
  }

  walk(root, 0);

  if (!count) {
    const empty = el("div", "notes-chat__option notes-chat__option--empty");
    empty.textContent = q ? "Ничего не найдено" : "Нечего прикрепить";
    menu.appendChild(empty);
  }
  return count;
}
