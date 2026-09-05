import { EditorView, keymap, placeholder } from "@codemirror/view";
import { EditorState, Compartment } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { Strikethrough } from "@lezer/markdown";
import { commands } from "./commands.js";
import { livePreview, livePreviewKeymap } from "./live.js";
import { noteTheme } from "./theme.js";

export function mountMarkdownCM(parent, { onChange } = {}) {
  const editable = new Compartment();
  let skipChange = false;

  const view = new EditorView({
    parent,
    state: EditorState.create({
      doc: "",
      extensions: [
        noteTheme,
        history(),
        markdown({ extensions: [Strikethrough] }),
        livePreview,
        livePreviewKeymap,
        placeholder("Пишите заметку"),
        editable.of(EditorView.editable.of(true)),
        keymap.of([
          { key: "Mod-b", run: commands.bold },
          { key: "Mod-i", run: commands.italic },
          ...historyKeymap,
          ...defaultKeymap,
        ]),
        EditorView.lineWrapping,
        EditorView.contentAttributes.of({ spellcheck: "true" }),
        EditorView.updateListener.of((update) => {
          if (skipChange || !update.docChanged || !onChange) return;
          onChange(update.state.doc.toString());
        }),
      ],
    }),
  });

  return {
    setContent(text) {
      const next = text ?? "";
      if (view.state.doc.toString() === next) return;
      skipChange = true;
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: next },
      });
      skipChange = false;
    },
    getContent() {
      return view.state.doc.toString();
    },
    setReadOnly(on) {
      view.dispatch({
        effects: editable.reconfigure(EditorView.editable.of(!on)),
      });
    },
    focus() {
      view.focus();
    },
    run(name) {
      const cmd = commands[name];
      if (typeof cmd === "function") cmd(view);
    },
    destroy() {
      view.destroy();
    },
  };
}
