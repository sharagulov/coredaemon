function wrapSelection(before, after, placeholder) {
  return (view) => {
    const { from, to } = view.state.selection.main;
    const selected = from === to ? placeholder : view.state.sliceDoc(from, to);
    const beforeStart = from - before.length;
    const afterEnd = to + after.length;
    const already =
      beforeStart >= 0 &&
      afterEnd <= view.state.doc.length &&
      view.state.sliceDoc(beforeStart, from) === before &&
      view.state.sliceDoc(to, afterEnd) === after;

    if (already) {
      view.dispatch({
        changes: { from: beforeStart, to: afterEnd, insert: view.state.sliceDoc(from, to) },
        selection: {
          anchor: beforeStart,
          head: beforeStart + (to - from),
        },
      });
    } else {
      view.dispatch({
        changes: { from, to, insert: before + selected + after },
        selection: {
          anchor: from + before.length,
          head: from + before.length + selected.length,
        },
      });
    }
    view.focus();
    return true;
  };
}

function toggleLinePrefix(prefix) {
  return (view) => {
    const { from, to } = view.state.selection.main;
    const start = view.state.doc.lineAt(from);
    const end = view.state.doc.lineAt(to);
    const changes = [];
    for (let n = start.number; n <= end.number; n++) {
      const line = view.state.doc.line(n);
      if (line.text.startsWith(prefix)) {
        changes.push({ from: line.from, to: line.from + prefix.length, insert: "" });
      } else {
        changes.push({ from: line.from, insert: prefix });
      }
    }
    view.dispatch({ changes });
    view.focus();
    return true;
  };
}

export const commands = {
  bold: wrapSelection("**", "**", "текст"),
  italic: wrapSelection("*", "*", "текст"),
  underline: wrapSelection("++", "++", "текст"),
  strike: wrapSelection("~~", "~~", "текст"),
  code: wrapSelection("`", "`", "код"),
  heading: toggleLinePrefix("# "),
  list: toggleLinePrefix("- "),
  quote: toggleLinePrefix("> "),
  link(view) {
    const url = window.prompt("URL", "https://");
    if (!url) {
      view.focus();
      return false;
    }
    return wrapSelection("[", `](${url})`, "текст")(view);
  },
};
