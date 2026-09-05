function escapeHtml(text) {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function stripToolMarkup(text) {
  return text
    .replace(/<tool_call>[\s\S]*?<\/tool_call>/gi, "")
    .replace(/<\/?tool_call>/gi, "")
    .trim();
}

function langLabel(raw) {
  const s = (raw || "").trim();
  if (!s) return "Code";
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function inlineMarkdown(text) {
  let html = escapeHtml(text);
  html = html.replace(/`([^`]+)`/g, '<code class="md-code">$1</code>');
  html = html.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  html = html.replace(/__([^_]+)__/g, "<strong>$1</strong>");
  html = html.replace(/~~([^~]+)~~/g, "<s>$1</s>");
  html = html.replace(/\+\+([^+]+)\+\+/g, '<u class="md-underline">$1</u>');
  html = html.replace(/(^|[\s(])\*([^*\n]+)\*(?=[\s).,]|$)/g, "$1<em>$2</em>");
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a class="md-link" href="$2">$1</a>');
  return html;
}

function renderCodeBlock(lang, code, copyIcon) {
  const block = document.createElement("div");
  block.className = "md-codeblock";

  const head = document.createElement("div");
  head.className = "md-codeblock__head";

  const chip = document.createElement("div");
  chip.className = "md-codeblock__chip";
  chip.textContent = langLabel(lang);

  const copyBtn = document.createElement("button");
  copyBtn.type = "button";
  copyBtn.className = "md-codeblock__copy";
  copyBtn.setAttribute("aria-label", "Копировать");
  copyBtn.innerHTML = `<img src="${copyIcon}" alt="">`;
  copyBtn.addEventListener("click", () => {
    navigator.clipboard.writeText(code).catch(() => {});
  });

  head.append(chip, copyBtn);

  const pre = document.createElement("pre");
  pre.className = "md-codeblock__pre";
  const codeEl = document.createElement("code");
  codeEl.textContent = code.replace(/\n$/, "");
  pre.appendChild(codeEl);

  block.append(head, pre);
  return block;
}

function renderLine(line) {
  const heading = line.match(/^(#{1,3})[ \t]+(.+)$/);
  if (heading) {
    const el = document.createElement("h2");
    el.className = "md-heading";
    el.innerHTML = inlineMarkdown(heading[2]);
    return el;
  }

  const list = line.match(/^[-*][ \t]+(.+)$/);
  if (list) {
    const row = document.createElement("div");
    row.className = "md-line md-line--list";
    const bullet = document.createElement("span");
    bullet.className = "md-line__bullet";
    bullet.textContent = "•";
    const text = document.createElement("span");
    text.className = "md-line__text";
    text.innerHTML = inlineMarkdown(list[1]);
    row.append(bullet, text);
    return row;
  }

  const quote = line.match(/^>[ \t]?(.*)$/);
  if (quote) {
    const row = document.createElement("blockquote");
    row.className = "md-line md-line--quote";
    row.innerHTML = inlineMarkdown(quote[1]);
    return row;
  }

  const row = document.createElement("p");
  row.className = "md-line md-line--para";
  row.innerHTML = inlineMarkdown(line);
  return row;
}

export function renderMarkdown(el, text, { copyIcon = "assets/icon-copy.svg" } = {}) {
  const source = stripToolMarkup(text || "");
  el.innerHTML = "";
  if (!source.trim()) {
    el.classList.add("md-preview--empty");
    el.textContent = "Пустая заметка";
    return;
  }
  el.classList.remove("md-preview--empty");

  const parts = source.split(/(```[^\n]*\n[\s\S]*?```)/g);
  for (const part of parts) {
    if (!part) continue;
    const fence = part.match(/^```([^\n]*)\n?([\s\S]*?)```$/);
    if (fence) {
      el.appendChild(renderCodeBlock(fence[1], fence[2], copyIcon));
      continue;
    }
    for (const line of part.split("\n")) {
      if (!line.trim()) continue;
      el.appendChild(renderLine(line));
    }
  }
}
