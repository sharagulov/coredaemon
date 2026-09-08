import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { createMenuOption } from "../menu-option.js";
import { bindSearchInput, createInlineForm } from "../input.js";
import { registerPopupDismiss } from "../popups.js";
import { createContextMenu, createContextAction } from "../context-menu.js";
import { createConfirmModal } from "../modal.js";
import { createDriveCrumbs } from "./crumbs.js";
import { createDriveRow, driveFileUrl, driveParentPath } from "./row.js";
import { createDrivePreview } from "./preview.js";
import { fromFileList, collectDriveDrop } from "./upload.js";

const SORTS = [
  { id: "name", label: "имени" },
  { id: "mtime", label: "дате изменения" },
];

function joinDrive(dir, name) {
  return dir ? `${dir}/${name}` : name;
}

/**
 * @param {{ crumbsHost: Element }} opts
 */
export function createDriveExplorer({ crumbsHost }) {
  let path = "";
  let entries = [];
  let query = "";
  let sortBy = "name";
  const selected = new Set();
  let focused = "";
  let anchor = "";

  const crumbs = createDriveCrumbs({ host: crumbsHost, onGo: setPath });

  const root = el("div", "drive-main");
  const explorer = el("section", "drive-explorer", { "aria-label": "Облако" });
  const preview = createDrivePreview();
  const deleteModal = createConfirmModal();
  document.body.appendChild(deleteModal.el);

  const toolbar = el("div", "drive-toolbar");
  const left = el("div", "drive-toolbar__left");
  const backBtn = el("button", "notes-bar notes-bar--dropdown drive-back", {
    type: "button",
    "aria-label": "Назад",
  });
  backBtn.appendChild(icon("assets/icon-return.png", "notes-bar__icon-slot notes-bar__icon-slot--return"));
  const backLabel = el("span");
  backLabel.textContent = "Назад";
  backBtn.appendChild(backLabel);
  backBtn.disabled = true;

  const searchLabel = el("label", "notes-bar notes-bar--search");
  searchLabel.appendChild(icon("assets/icon-search.png", "notes-bar__icon-slot notes-bar__icon-slot--search"));
  const searchInput = el("input", "notes-bar__search-input", {
    type: "search",
    placeholder: "Поиск",
    autocomplete: "off",
    spellcheck: "false",
    "aria-label": "Поиск по папке и вложенным файлам",
  });
  searchLabel.appendChild(searchInput);

  const addWrap = el("div", "drive-add");
  const addBtn = el("button", "notes-bar notes-bar--short", {
    type: "button",
    "aria-label": "Добавить",
    "aria-haspopup": "menu",
    "aria-expanded": "false",
  });
  addBtn.appendChild(icon("assets/icon-plus.png", "notes-bar__icon-slot notes-bar__icon-slot--plus"));
  const addMenu = el("div", "notes-sort__menu drive-add__menu", { hidden: "", role: "menu" });
  addWrap.append(addBtn, addMenu);

  const fileInput = el("input", "drive-file-input", {
    type: "file",
    multiple: "",
    tabindex: "-1",
    "aria-hidden": "true",
  });
  const folderInput = el("input", "drive-file-input", {
    type: "file",
    multiple: "",
    tabindex: "-1",
    "aria-hidden": "true",
  });
  folderInput.setAttribute("webkitdirectory", "");
  folderInput.setAttribute("directory", "");

  const sortWrap = el("div", "notes-sort drive-sort");
  const sortBtn = el("button", "notes-bar notes-bar--dropdown", {
    type: "button",
    "aria-label": "Сортировка",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
  });
  const sortMuted = el("span", "notes-bar__sort-muted");
  sortMuted.textContent = "Сортировать по ";
  const sortLabel = el("span");
  sortLabel.textContent = "имени";
  sortBtn.append(sortMuted, sortLabel, icon("assets/icon-chevron.png", "notes-bar__icon-slot notes-bar__icon-slot--chevron"));
  const sortMenu = el("div", "notes-sort__menu", { hidden: "", role: "listbox" });
  for (const opt of SORTS) {
    sortMenu.appendChild(createMenuOption({
      className: "notes-sort__option",
      id: opt.id,
      label: opt.label,
      active: opt.id === sortBy,
      dataKey: "sort",
    }));
  }
  sortWrap.append(sortBtn, sortMenu);

  left.append(backBtn, searchLabel, addWrap);
  toolbar.append(left, sortWrap);

  const grid = el("div", "drive-grid", { tabindex: "0" });
  explorer.append(toolbar, grid, fileInput, folderInput);
  root.append(explorer, preview.el);

  function paintAddMenu() {
    addMenu.innerHTML = "";
    addMenu.append(
      createMenuOption({ className: "notes-sort__option", id: "folder", label: "Папка", dataKey: "action" }),
      createMenuOption({ className: "notes-sort__option", id: "upload", label: "Загрузить", dataKey: "action" }),
      createMenuOption({ className: "notes-sort__option", id: "upload-folder", label: "Загрузить папку", dataKey: "action" }),
    );
  }

  function showFolderForm() {
    addMenu.innerHTML = "";
    addMenu.appendChild(createInlineForm({
      formClass: "notes-filter__form",
      inputClass: "notes-filter__input",
      cancelClass: "notes-filter__icon-btn",
      confirmClass: "notes-filter__icon-btn notes-filter__icon-btn--confirm",
      placeholder: "Название папки",
      maxLength: 200,
      onConfirm: (name) => {
        add.close();
        mkdir(name);
      },
      onCancel: () => {
        paintAddMenu();
      },
    }));
    addMenu.hidden = false;
  }

  function visible() {
    const list = [...entries];
    list.sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
      if (sortBy === "mtime") return (b.mtime || 0) - (a.mtime || 0);
      return a.name.localeCompare(b.name, "ru", { sensitivity: "base" });
    });
    return list;
  }

  function rangePaths(list, from, to) {
    const a = list.findIndex((item) => item.path === from);
    const b = list.findIndex((item) => item.path === to);
    if (a < 0 && b < 0) return [];
    if (a < 0) return [to];
    if (b < 0) return [from];
    const lo = Math.min(a, b);
    const hi = Math.max(a, b);
    return list.slice(lo, hi + 1).map((item) => item.path);
  }

  function clearSelection() {
    selected.clear();
    focused = "";
    anchor = "";
    preview.clear();
    syncSelection();
  }

  function pruneSelection(list) {
    const paths = new Set(list.map((item) => item.path));
    for (const rel of [...selected]) {
      if (!paths.has(rel)) selected.delete(rel);
    }
    if (focused && !paths.has(focused)) focused = "";
    if (anchor && !paths.has(anchor)) anchor = focused;
    if (!focused && selected.size) focused = [...selected][0];
  }

  function tileByPath(rel) {
    if (!rel) return null;
    for (const node of grid.querySelectorAll(".drive-tile")) {
      if (node.dataset.path === rel) return node;
    }
    return null;
  }

  function syncSelection() {
    const tiles = [...grid.querySelectorAll(".drive-tile")];
    for (const btn of tiles) {
      const rel = btn.dataset.path;
      const isFocused = rel === focused;
      btn.classList.toggle("is-selected", selected.has(rel));
      btn.classList.toggle("is-focused", isFocused);
      btn.tabIndex = isFocused ? 0 : -1;
    }
    if (!focused && tiles[0]) tiles[0].tabIndex = 0;
  }

  function focusTile(rel) {
    const btn = tileByPath(rel);
    if (!btn) return;
    btn.focus();
    btn.scrollIntoView({ block: "nearest", inline: "nearest" });
  }

  function gridColumns() {
    const tile = 140;
    const gap = 15;
    return Math.max(1, Math.floor((grid.clientWidth + gap) / (tile + gap)));
  }

  function renderGrid() {
    const keepFocus = document.activeElement
      && grid.contains(document.activeElement)
      && document.activeElement.closest(".drive-tile, .drive-grid");
    grid.replaceChildren();
    const list = visible();
    pruneSelection(list);
    if (!list.length) {
      const empty = el("div", "drive-grid__empty");
      empty.textContent = query.trim() ? "Ничего не найдено" : "Пусто";
      grid.appendChild(empty);
      return;
    }
    const searching = !!query.trim();
    for (const entry of list) {
      const hint = searching ? driveParentPath(entry.path) : "";
      grid.appendChild(createDriveRow(entry, {
        selected: selected.has(entry.path),
        focused: entry.path === focused,
        pathHint: hint && hint !== path ? hint : "",
        onSelect: selectEntry,
        onActivate: activateEntry,
        onContextMenu: openItemMenu,
      }));
    }
    syncSelection();
    if (keepFocus && focused) focusTile(focused);
  }

  function itemUrl(rel) {
    return "/api/drive/item/" + String(rel).split("/").map(encodeURIComponent).join("/");
  }

  async function removeEntries(list) {
    for (const entry of list) {
      const res = await fetch(itemUrl(entry.path), { method: "DELETE" });
      if (!res.ok) {
        showErr("Не удалось удалить");
        await load();
        return;
      }
      selected.delete(entry.path);
      if (focused === entry.path) focused = "";
      if (anchor === entry.path) anchor = focused;
    }
    preview.clear();
    await load();
  }

  function askDelete(list) {
    if (!list.length) return;
    const one = list.length === 1 ? list[0] : null;
    const allDirs = list.every((item) => item.is_dir);
    const allFiles = list.every((item) => !item.is_dir);
    deleteModal.open({
      title: one
        ? (one.is_dir ? "Удалить папку?" : "Удалить файл?")
        : allDirs
          ? "Удалить папки?"
          : allFiles
            ? "Удалить файлы?"
            : "Удалить элементы?",
      message: one
        ? (one.is_dir
          ? `«${one.name}» и всё содержимое будут удалены безвозвратно.`
          : `«${one.name}» будет удалён безвозвратно.`)
        : "Выбранные файлы и папки будут удалены безвозвратно.",
      confirmLabel: "Удалить",
      onConfirm: () => removeEntries(list),
    });
  }

  function openItemMenu(e, entry) {
    if (!selected.has(entry.path)) {
      selected.clear();
      selected.add(entry.path);
      focused = entry.path;
      anchor = entry.path;
      syncSelection();
    }
    const targets = visible().filter((item) => selected.has(item.path));
    ctx.openAt(e, (menu) => {
      menu.appendChild(createContextAction({
        label: "Удалить",
        danger: true,
        onClick: () => {
          ctx.close();
          askDelete(targets.length ? targets : [entry]);
        },
      }));
    });
  }

  function selectEntry(e, entry) {
    const list = visible();
    const rel = entry.path;
    const ctrl = e.ctrlKey || e.metaKey;
    const shift = e.shiftKey;
    if (shift && (anchor || focused)) {
      const from = anchor || focused;
      const range = rangePaths(list, from, rel);
      if (ctrl) {
        for (const p of range) selected.add(p);
      } else {
        selected.clear();
        for (const p of range) selected.add(p);
      }
      focused = rel;
    } else if (ctrl) {
      if (selected.has(rel)) selected.delete(rel);
      else selected.add(rel);
      focused = rel;
      anchor = rel;
    } else {
      selected.clear();
      selected.add(rel);
      focused = rel;
      anchor = rel;
    }
    syncSelection();
    focusTile(rel);
  }

  function activateEntry(entry) {
    if (!entry) return;
    selected.clear();
    selected.add(entry.path);
    focused = entry.path;
    anchor = entry.path;
    if (entry.is_dir) {
      preview.clear();
      setPath(entry.path);
      return;
    }
    syncSelection();
    if (entry.kind === "image" || entry.kind === "video") {
      preview.show(entry);
      return;
    }
    preview.clear();
    const a = el("a", "", { href: driveFileUrl(entry.path), download: entry.name });
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  function activateFocused() {
    const list = visible();
    const entry = list.find((item) => item.path === focused)
      || (selected.size === 1 ? list.find((item) => selected.has(item.path)) : null);
    if (entry) activateEntry(entry);
  }

  function moveFocus(delta, { extend = false, additive = false } = {}) {
    const list = visible();
    if (!list.length) return;
    let i = list.findIndex((item) => item.path === focused);
    if (i < 0) {
      i = 0;
    } else {
      if (extend && !anchor) anchor = focused;
      i = Math.max(0, Math.min(list.length - 1, i + delta));
    }
    const entry = list[i];
    focused = entry.path;
    if (extend) {
      const from = anchor || focused;
      selected.clear();
      for (const p of rangePaths(list, from, focused)) selected.add(p);
    } else if (!additive) {
      selected.clear();
      selected.add(focused);
      anchor = focused;
    }
    syncSelection();
    focusTile(focused);
  }

  function onGridKey(e) {
    if (e.target.closest(".drive-toolbar, input, textarea")) return;
    const cols = gridColumns();
    if (e.key === "Enter") {
      e.preventDefault();
      activateFocused();
      return;
    }
    let delta = 0;
    if (e.key === "ArrowLeft") delta = -1;
    else if (e.key === "ArrowRight") delta = 1;
    else if (e.key === "ArrowUp") delta = -cols;
    else if (e.key === "ArrowDown") delta = cols;
    else return;
    e.preventDefault();
    moveFocus(delta, {
      extend: e.shiftKey,
      additive: e.ctrlKey || e.metaKey,
    });
  }

  async function load() {
    crumbs.render(path);
    const params = new URLSearchParams();
    if (path) params.set("path", path);
    const q = query.trim();
    if (q) params.set("q", q);
    try {
      const res = await fetch(`/api/drive/list?${params}`);
      if (!res.ok) {
        let message = res.statusText;
        try {
          const body = await res.json();
          if (body.error) message = body.error;
        } catch {
          // keep statusText
        }
        throw new Error(message);
      }
      entries = await res.json();
      if (!Array.isArray(entries)) entries = [];
      renderGrid();
    } catch (err) {
      entries = [];
      grid.replaceChildren();
      const empty = el("div", "drive-grid__empty");
      empty.textContent = err.message || "Ошибка";
      grid.appendChild(empty);
    }
  }

  function showErr(message) {
    grid.replaceChildren();
    const empty = el("div", "drive-grid__empty");
    empty.textContent = message || "Ошибка";
    grid.appendChild(empty);
  }

  async function mkdir(name) {
    const dest = joinDrive(path, name.trim());
    const res = await fetch("/api/drive/mkdir", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: dest }),
    });
    if (!res.ok) {
      showErr("Не удалось создать папку");
      return;
    }
    await load();
  }

  async function uploadItems(files, dirs = []) {
    const kept = (files || []).filter((item) => item?.file && item.rel);
    const keptDirs = (dirs || []).filter(Boolean);
    if (!kept.length && !keptDirs.length) return;
    const fd = new FormData();
    fd.append("path", path);
    for (const dir of keptDirs) fd.append("dir", dir);
    for (const item of kept) {
      fd.append("rel", item.rel);
      fd.append("file", item.file);
    }
    const res = await fetch("/api/drive/upload", { method: "POST", body: fd });
    fileInput.value = "";
    folderInput.value = "";
    if (!res.ok) {
      showErr("Не удалось загрузить");
      return;
    }
    await load();
  }

  function goUp() {
    if (!path) return;
    setPath(driveParentPath(path));
  }

  function setPath(next) {
    path = next || "";
    selected.clear();
    focused = "";
    anchor = "";
    query = "";
    searchInput.value = "";
    preview.clear();
    backBtn.disabled = !path;
    load();
  }

  paintAddMenu();

  const add = createDropdown({
    trigger: addBtn,
    menu: addMenu,
    optionSelector: ".notes-sort__option",
    dataKey: "action",
    rootSelector: ".drive-add",
    onSelect: (value) => {
      if (value === "folder") {
        showFolderForm();
        return;
      }
      if (value === "upload") {
        fileInput.click();
        add.close();
      }
      if (value === "upload-folder") {
        folderInput.click();
        add.close();
      }
    },
    onClose: paintAddMenu,
  });

  const sort = createDropdown({
    trigger: sortBtn,
    menu: sortMenu,
    optionSelector: ".notes-sort__option",
    dataKey: "sort",
    rootSelector: ".drive-sort",
    onSelect: (value) => {
      sortBy = value;
      sortLabel.textContent = SORTS.find((s) => s.id === value)?.label || value;
      for (const btn of sortMenu.querySelectorAll(".notes-sort__option")) {
        const active = btn.dataset.sort === value;
        btn.classList.toggle("is-active", active);
      }
      sort.close();
      renderGrid();
    },
  });

  backBtn.addEventListener("click", goUp);

  bindSearchInput(searchInput, {
    debounceMs: 150,
    onSearch: (value) => {
      query = value;
      load();
    },
    onClear: () => {
      query = "";
      load();
    },
  });

  const ctxRoot = el("div", "notes-ctx drive-ctx", { hidden: "", "aria-hidden": "true" });
  const ctxMenu = el("div", "notes-ctx__menu", { role: "menu" });
  ctxRoot.appendChild(ctxMenu);
  document.body.appendChild(ctxRoot);
  const ctx = createContextMenu({ root: ctxRoot, menu: ctxMenu });
  ctx.mount();
  document.addEventListener("contextmenu", (e) => {
    if (!e.target.closest(".drive-tile") && !e.target.closest(".drive-ctx")) ctx.close();
  });

  fileInput.addEventListener("change", () => uploadItems(fromFileList(fileInput.files)));
  folderInput.addEventListener("change", () => uploadItems(fromFileList(folderInput.files)));
  grid.addEventListener("click", (e) => {
    if (e.target.closest(".drive-tile")) return;
    clearSelection();
  });
  explorer.addEventListener("keydown", onGridKey);
  explorer.addEventListener("dragover", (e) => {
    e.preventDefault();
    explorer.classList.add("is-drop");
  });
  explorer.addEventListener("dragleave", (e) => {
    if (!explorer.contains(e.relatedTarget)) explorer.classList.remove("is-drop");
  });
  explorer.addEventListener("drop", (e) => {
    e.preventDefault();
    explorer.classList.remove("is-drop");
    collectDriveDrop(e.dataTransfer).then(({ files, dirs }) => uploadItems(files, dirs));
  });
  registerPopupDismiss([
    { rootSelector: ".drive-add", close: add.close },
    { rootSelector: ".drive-sort", close: sort.close },
    { rootSelector: ".drive-ctx", close: ctx.close },
  ]);
  setInterval(() => load(), 5000);

  return {
    el: root,
    reload: load,
  };
}
