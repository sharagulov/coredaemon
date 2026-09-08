import { el, icon } from "../dom.js";
import { createDropdown } from "../dropdown.js";
import { createMenuOption } from "../menu-option.js";
import { bindSearchInput, createInlineForm } from "../input.js";
import { registerPopupDismiss } from "../popups.js";
import { createContextMenu, createContextAction } from "../context-menu.js";
import { createDriveCrumbs } from "./crumbs.js";
import { createDriveRow, driveFileUrl, driveParentPath } from "./row.js";
import { createDrivePreview } from "./preview.js";

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
  let selected = "";

  const crumbs = createDriveCrumbs({ host: crumbsHost, onGo: setPath });

  const root = el("div", "drive-main");
  const explorer = el("section", "drive-explorer", { "aria-label": "Облако" });
  const preview = createDrivePreview();

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

  const grid = el("div", "drive-grid");
  explorer.append(toolbar, grid, fileInput);
  root.append(explorer, preview.el);

  function paintAddMenu() {
    addMenu.innerHTML = "";
    addMenu.append(
      createMenuOption({ className: "notes-sort__option", id: "folder", label: "Папка", dataKey: "action" }),
      createMenuOption({ className: "notes-sort__option", id: "upload", label: "Загрузить", dataKey: "action" }),
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

  function renderGrid() {
    grid.replaceChildren();
    const list = visible();
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
        selected: entry.path === selected,
        pathHint: hint && hint !== path ? hint : "",
        onOpen: openEntry,
        onContextMenu: openItemMenu,
      }));
    }
  }

  function itemUrl(rel) {
    return "/api/drive/item/" + String(rel).split("/").map(encodeURIComponent).join("/");
  }

  async function removeEntry(entry) {
    const res = await fetch(itemUrl(entry.path), { method: "DELETE" });
    if (!res.ok) {
      showErr("Не удалось удалить");
      return;
    }
    if (selected === entry.path) {
      selected = "";
      preview.clear();
    }
    await load();
  }

  function openItemMenu(e, entry) {
    ctx.openAt(e, (menu) => {
      menu.appendChild(createContextAction({
        label: "Удалить",
        danger: true,
        onClick: () => {
          ctx.close();
          removeEntry(entry);
        },
      }));
    });
  }

  function openEntry(entry) {
    selected = entry.path;
    renderGrid();
    if (entry.is_dir) {
      preview.clear();
      setPath(entry.path);
      return;
    }
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

  async function upload(files) {
    if (!files.length) return;
    const fd = new FormData();
    fd.append("path", path);
    for (const file of files) fd.append("file", file);
    const res = await fetch("/api/drive/upload", { method: "POST", body: fd });
    fileInput.value = "";
    if (!res.ok) {
      showErr("Не удалось загрузить файл");
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
    selected = "";
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

  fileInput.addEventListener("change", () => upload([...fileInput.files]));
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
    const files = [...(e.dataTransfer?.files || [])];
    if (files.length) upload(files);
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
