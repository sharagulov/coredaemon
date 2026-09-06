import { EditorView, keymap, placeholder } from "@codemirror/view";
import { EditorState, Compartment } from "@codemirror/state";
import {
  defaultKeymap,
  history,
  historyKeymap,
  redo,
  redoDepth,
  undo,
  undoDepth,
} from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { Strikethrough } from "@lezer/markdown";
import { commands } from "./commands.js";
import { livePreview, livePreviewKeymap } from "./live.js";
import { noteTheme } from "./theme.js";

const historyCmds = { undo, redo };

function historyFlags(state) {
  return { undo: undoDepth(state) > 0, redo: redoDepth(state) > 0 };
}

export function mountMarkdownCM(parent, { onChange, onHistory } = {}) {
  const editable = new Compartment();
  let skipChange = false;
  let readOnly = false;

  function extensions() {
    return [
      noteTheme,
      history(),
      markdown({ extensions: [Strikethrough] }),
      livePreview,
      livePreviewKeymap,
      placeholder("Пишите заметку"),
      editable.of(EditorView.editable.of(!readOnly)),
      keymap.of([
        { key: "Mod-b", run: commands.bold, preventDefault: true },
        { key: "Mod-i", run: commands.italic, preventDefault: true },
        { key: "Mod-u", run: commands.underline, preventDefault: true },
        { key: "Mod-Shift-x", run: commands.strike, preventDefault: true },
        { key: "Mod-k", run: commands.link, preventDefault: true },
        { key: "Mod-e", run: commands.code, preventDefault: true },
        { key: "Mod-Shift-h", run: commands.heading, preventDefault: true },
        { key: "Mod-Shift-l", run: commands.list, preventDefault: true },
        { key: "Mod-Shift-.", run: commands.quote, preventDefault: true },
        ...historyKeymap,
        ...defaultKeymap,
      ]),
      EditorView.lineWrapping,
      EditorView.contentAttributes.of({ spellcheck: "true" }),
      EditorView.updateListener.of((update) => {
        if (!skipChange && update.docChanged && onChange) {
          onChange(update.state.doc.toString());
        }
        if (onHistory) onHistory(historyFlags(update.state));
      }),
    ];
  }

  const view = new EditorView({
    parent,
    state: EditorState.create({ doc: "", extensions: extensions() }),
  });

  return {
    setContent(text) {
      skipChange = true;
      view.setState(EditorState.create({ doc: text ?? "", extensions: extensions() }));
      skipChange = false;
      if (onHistory) onHistory(historyFlags(view.state));
    },
    getContent() {
      return view.state.doc.toString();
    },
    setReadOnly(on) {
      readOnly = on;
      view.dispatch({
        effects: editable.reconfigure(EditorView.editable.of(!on)),
      });
    },
    focus() {
      view.focus();
    },
    run(name) {
      const hist = historyCmds[name];
      if (hist) {
        hist(view);
        return;
      }
      const cmd = commands[name];
      if (typeof cmd === "function") cmd(view);
    },
    destroy() {
      view.destroy();
    },
  };
}
