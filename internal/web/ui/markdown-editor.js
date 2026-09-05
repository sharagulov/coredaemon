import { mountMarkdownCM } from "../cm-bundle.js";

const TOOLBAR = [
  { id: "bold", label: "Жирный", icon: "assets/editor-bold.svg" },
  { id: "italic", label: "Курсив", icon: "assets/editor-italic.svg" },
  { id: "underline", label: "Подчёркнутый", icon: "assets/editor-underline.svg" },
  { id: "strike", label: "Зачёркнутый", icon: "assets/editor-strikethrough.svg" },
  { id: "link", label: "Ссылка", icon: "assets/editor-link.svg" },
  { id: "code", label: "Код", icon: "assets/editor-code.svg" },
  { id: "heading", label: "Заголовок", icon: "assets/editor-type.svg" },
  { id: "list", label: "Список", icon: "assets/editor-list.svg" },
  { id: "quote", label: "Цитата", icon: "assets/editor-quote.svg" },
];

function createToolButton({ label, iconSrc, onClick }) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "notes-editor__tool";
  btn.setAttribute("aria-label", label);
  const slot = document.createElement("span");
  slot.className = "notes-editor__tool-icon";
  const img = document.createElement("img");
  img.src = iconSrc;
  img.alt = "";
  slot.appendChild(img);
  btn.appendChild(slot);
  btn.addEventListener("mousedown", (e) => e.preventDefault());
  btn.addEventListener("click", (e) => {
    e.preventDefault();
    onClick();
  });
  return btn;
}

export function createMarkdownEditor({ panel, onChange }) {
  const toolbar = panel.querySelector(".notes-editor__toolbar");
  const mount = panel.querySelector(".notes-editor__cm") || panel.querySelector(".notes-editor__area");
  const cm = mountMarkdownCM(mount, { onChange });

  for (const item of TOOLBAR) {
    toolbar.appendChild(
      createToolButton({
        label: item.label,
        iconSrc: item.icon,
        onClick: () => cm.run(item.id),
      }),
    );
  }

  return {
    setContent(text) {
      cm.setContent(text || "");
    },
    getContent() {
      return cm.getContent();
    },
    setReadOnly(on) {
      toolbar.hidden = on;
      cm.setReadOnly(on);
    },
    focus() {
      cm.focus();
    },
  };
}
