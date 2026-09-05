import { Decoration, EditorView, ViewPlugin, WidgetType } from "@codemirror/view";
import { RangeSetBuilder } from "@codemirror/state";
import { syntaxTree } from "@codemirror/language";

class BulletWidget extends WidgetType {
  toDOM() {
    const el = document.createElement("span");
    el.className = "cm-md-bullet";
    el.textContent = "•";
    return el;
  }
  eq() {
    return true;
  }
  ignoreEvent() {
    return true;
  }
}

const hide = Decoration.replace({});
const bullet = Decoration.replace({ widget: new BulletWidget() });

function mark(cls) {
  return Decoration.mark({ class: cls });
}

function line(cls) {
  return Decoration.line({ class: cls });
}

function touches(sel, from, to) {
  for (const r of sel.ranges) {
    if (r.from <= to && r.to >= from) return true;
  }
  return false;
}

function lineTouches(state, sel, from, to) {
  for (const r of sel.ranges) {
    const a = state.doc.lineAt(r.from);
    const b = state.doc.lineAt(r.to);
    if (a.from <= to && b.to >= from) return true;
  }
  return false;
}

function eatSpace(state, pos) {
  return state.sliceDoc(pos, pos + 1) === " " ? pos + 1 : pos;
}

function add(items, from, to, deco) {
  if (from < to) items.push({ from, to, deco });
}

function addLine(items, pos, cls) {
  items.push({ from: pos, to: pos, deco: line(cls), line: true });
}

function hideMarks(node, markName, items, sel, cls) {
  const marks = [];
  for (let c = node.node.firstChild; c; c = c.nextSibling) {
    if (c.name === markName) marks.push(c);
  }
  const innerFrom = marks[0] ? marks[0].to : node.from;
  const innerTo = marks.length ? marks[marks.length - 1].from : node.to;
  if (cls && innerFrom < innerTo) add(items, innerFrom, innerTo, mark(cls));
  if (!touches(sel, node.from, node.to)) {
    for (const m of marks) add(items, m.from, m.to, hide);
  }
}

function extraPairs(view, items, sel, re, cls, markLen) {
  const { from, to } = view.viewport;
  const text = view.state.sliceDoc(from, to);
  re.lastIndex = 0;
  let m;
  while ((m = re.exec(text))) {
    const a = from + m.index;
    const b = a + m[0].length;
    add(items, a + markLen, b - markLen, mark(cls));
    if (!touches(sel, a, b)) {
      add(items, a, a + markLen, hide);
      add(items, b - markLen, b, hide);
    }
  }
}

function build(view) {
  const items = [];
  const sel = view.state.selection;
  const state = view.state;

  syntaxTree(state).iterate({
    from: view.viewport.from,
    to: view.viewport.to,
    enter(node) {
      const name = node.name;

      if (name.startsWith("ATXHeading")) {
        const level = name.replace(/\D/g, "") || "1";
        const row = state.doc.lineAt(node.from);
        addLine(items, row.from, `cm-md-heading cm-md-h${level}`);
        let innerFrom = node.from;
        for (let c = node.node.firstChild; c; c = c.nextSibling) {
          if (c.name === "HeaderMark") {
            innerFrom = eatSpace(state, c.to);
            if (!touches(sel, node.from, node.to)) add(items, c.from, innerFrom, hide);
          }
        }
        if (innerFrom < node.to) add(items, innerFrom, node.to, mark(`cm-md-heading cm-md-h${level}`));
        return false;
      }

      if (name === "StrongEmphasis") {
        hideMarks(node, "EmphasisMark", items, sel, "cm-md-strong");
        return false;
      }
      if (name === "Emphasis") {
        hideMarks(node, "EmphasisMark", items, sel, "cm-md-em");
        return false;
      }
      if (name === "Strikethrough") {
        hideMarks(node, "StrikethroughMark", items, sel, "cm-md-strike");
        return false;
      }
      if (name === "InlineCode") {
        hideMarks(node, "CodeMark", items, sel, "cm-md-code");
        return false;
      }

      if (name === "QuoteMark") {
        const row = state.doc.lineAt(node.from);
        addLine(items, row.from, "cm-md-quote");
        const bodyFrom = eatSpace(state, node.to);
        if (!lineTouches(state, sel, row.from, row.to)) add(items, node.from, bodyFrom, hide);
        if (bodyFrom < row.to) add(items, bodyFrom, row.to, mark("cm-md-quote-text"));
        return;
      }

      if (name === "ListMark") {
        const row = state.doc.lineAt(node.from);
        addLine(items, row.from, "cm-md-list");
        if (!lineTouches(state, sel, row.from, row.to)) {
          add(items, node.from, eatSpace(state, node.to), bullet);
        }
        return;
      }

      if (name === "FencedCode") {
        const start = state.doc.lineAt(node.from);
        const end = state.doc.lineAt(node.to);
        for (let n = start.number; n <= end.number; n++) {
          addLine(items, state.doc.line(n).from, "cm-md-fence");
        }
        if (!touches(sel, node.from, node.to)) {
          add(items, start.from, start.to, hide);
          if (end.number !== start.number) add(items, end.from, end.to, hide);
        }
        return false;
      }

      if (name === "Link") {
        const marks = [];
        let url = null;
        for (let c = node.node.firstChild; c; c = c.nextSibling) {
          if (c.name === "LinkMark") marks.push(c);
          if (c.name === "URL") url = c;
        }
        if (marks.length >= 2) add(items, marks[0].to, marks[1].from, mark("cm-md-link"));
        if (!touches(sel, node.from, node.to)) {
          for (const m of marks) add(items, m.from, m.to, hide);
          if (url) add(items, url.from, url.to, hide);
        }
        return false;
      }
    },
  });

  extraPairs(view, items, sel, /\+\+([^+]+)\+\+/g, "cm-md-underline", 2);

  items.sort((a, b) => a.from - b.from || a.to - b.to);
  const builder = new RangeSetBuilder();
  let lastFrom = -1;
  let lastTo = -1;
  for (const item of items) {
    if (item.line) {
      builder.add(item.from, item.to, item.deco);
      continue;
    }
    if (item.from < lastTo && item.from >= lastFrom) continue;
    builder.add(item.from, item.to, item.deco);
    lastFrom = item.from;
    lastTo = item.to;
  }
  return builder.finish();
}

function hrefAt(view, pos) {
  let href = "";
  syntaxTree(view.state).iterate({
    enter(node) {
      if (node.name === "Link" && node.from <= pos && node.to >= pos) {
        for (let c = node.node.firstChild; c; c = c.nextSibling) {
          if (c.name === "URL") href = view.state.sliceDoc(c.from, c.to);
        }
      }
    },
  });
  return href;
}

function openHref(href) {
  try {
    const url = new URL(href, window.location.origin);
    if (url.protocol !== "http:" && url.protocol !== "https:") return;
    window.open(url.href, "_blank", "noopener,noreferrer");
  } catch {
    // ignore
  }
}

export const livePreview = ViewPlugin.fromClass(
  class {
    constructor(view) {
      this.decorations = build(view);
    }
    update(update) {
      if (update.docChanged || update.selectionSet || update.viewportChanged) {
        this.decorations = build(update.view);
      }
    }
  },
  { decorations: (v) => v.decorations },
);

export const livePreviewKeymap = EditorView.domEventHandlers({
  click(event, view) {
    if (!event.ctrlKey && !event.metaKey) return false;
    const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
    if (pos == null) return false;
    const href = hrefAt(view, pos);
    if (!href) return false;
    event.preventDefault();
    openHref(href);
    return true;
  },
});
