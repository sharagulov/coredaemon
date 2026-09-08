import { el, icon } from "../dom.js";
import { driveFileUrl } from "./row.js";

/**
 * @param {{ onClose?: () => void }} [opts]
 */
export function createDrivePreview({ onClose } = {}) {
  const root = el("aside", "drive-preview is-empty", { "aria-label": "Превью" });
  const bar = el("div", "drive-preview__bar");
  const closeBtn = el("button", "drive-preview__close notes-bar notes-bar--short", {
    type: "button",
    "aria-label": "Закрыть превью",
  });
  closeBtn.appendChild(icon("assets/icon-close.svg", "notes-bar__icon-slot notes-bar__icon-slot--close"));
  const spacer = el("div", "drive-preview__spacer");
  const label = el("div", "notes-bar notes-bar--dropdown drive-preview__tag");
  const text = el("span");
  text.textContent = "Превью";
  label.appendChild(text);
  bar.append(closeBtn, spacer, label);

  const body = el("div", "drive-preview__body");
  root.append(bar, body);

  function clear() {
    body.replaceChildren();
    root.classList.add("is-empty");
    root.classList.remove("is-open");
  }

  /** @param {{ path: string, name: string, kind?: string }} entry */
  function show(entry) {
    body.replaceChildren();
    root.classList.remove("is-empty");
    root.classList.add("is-open");
    const url = driveFileUrl(entry.path);
    if (entry.kind === "image") {
      const img = el("img", "drive-preview__media", { alt: entry.name, src: url });
      body.appendChild(img);
      return;
    }
    if (entry.kind === "video") {
      const video = el("video", "drive-preview__media", { controls: "", src: url });
      body.appendChild(video);
    }
  }

  function onEscape(e) {
    if (e.key !== "Escape" || !root.classList.contains("is-open")) return;
    e.preventDefault();
    clear();
    onClose?.();
  }

  closeBtn.addEventListener("click", () => {
    clear();
    onClose?.();
  });

  document.addEventListener("keydown", onEscape);

  return { el: root, show, clear };
}
