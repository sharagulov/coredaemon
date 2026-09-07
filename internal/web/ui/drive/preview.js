import { el } from "../dom.js";
import { driveFileUrl } from "./row.js";

export function createDrivePreview() {
  const root = el("aside", "drive-preview", { "aria-label": "Превью" });
  const bar = el("div", "drive-preview__bar");
  const spacer = el("div", "drive-preview__spacer");
  const label = el("div", "notes-bar notes-bar--dropdown drive-preview__tag");
  const text = el("span");
  text.textContent = "Превью";
  label.appendChild(text);
  bar.append(spacer, label);

  const body = el("div", "drive-preview__body");
  root.append(bar, body);

  function clear() {
    body.replaceChildren();
  }

  /** @param {{ path: string, name: string, kind?: string }} entry */
  function show(entry) {
    clear();
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

  return { el: root, show, clear };
}
