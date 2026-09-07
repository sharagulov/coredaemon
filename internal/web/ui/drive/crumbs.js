import { el } from "../dom.js";

/**
 * @param {{ host: Element, onGo: (path: string) => void }} opts
 */
export function createDriveCrumbs({ host, onGo }) {
  const nav = host;
  nav.classList.add("drive-crumbs");
  nav.setAttribute("aria-label", "Путь");

  function render(path) {
    nav.innerHTML = "";
    const parts = String(path || "").split("/").filter(Boolean);
    appendSeg("Облако", "", parts.length === 0);
    let acc = "";
    for (let i = 0; i < parts.length; i++) {
      acc = acc ? `${acc}/${parts[i]}` : parts[i];
      const sep = el("span", "drive-crumbs__sep");
      sep.textContent = "/";
      nav.appendChild(sep);
      appendSeg(parts[i], acc, i === parts.length - 1);
    }
  }

  function appendSeg(label, path, current) {
    const btn = el("button", "drive-crumbs__seg", { type: "button" });
    btn.textContent = label;
    btn.classList.toggle("is-current", current);
    btn.addEventListener("click", () => onGo(path));
    nav.appendChild(btn);
  }

  render("");
  return { el: nav, render };
}
