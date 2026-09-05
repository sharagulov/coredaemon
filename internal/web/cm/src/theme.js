import { EditorView } from "@codemirror/view";

export const noteTheme = EditorView.theme(
  {
    "&": {
      height: "100%",
      background: "#111",
      color: "#fff",
      fontSize: "15px",
    },
    "&.cm-focused": {
      outline: "none",
    },
    ".cm-scroller": {
      fontFamily: "Inter, system-ui, sans-serif",
      lineHeight: "1.5",
      padding: "16px",
    },
    ".cm-content": {
      caretColor: "#fff",
      padding: "0",
    },
    ".cm-gutters": {
      display: "none",
    },
    ".cm-activeLine": {
      backgroundColor: "transparent",
    },
    ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": {
      background: "rgba(255, 255, 255, 0.12)",
    },
    ".cm-cursor, .cm-dropCursor": {
      borderLeftColor: "#fff",
    },
    ".cm-md-heading": {
      fontWeight: "700",
      lineHeight: "normal",
    },
    ".cm-md-atxheading1": {
      fontSize: "24px",
    },
    ".cm-md-atxheading2": {
      fontSize: "22px",
    },
    ".cm-md-atxheading3": {
      fontSize: "20px",
    },
    ".cm-md-strong": {
      fontWeight: "700",
    },
    ".cm-md-em": {
      fontStyle: "italic",
    },
    ".cm-md-strike": {
      textDecoration: "line-through",
    },
    ".cm-md-underline": {
      textDecoration: "underline",
    },
    ".cm-md-code": {
      padding: "2px 6px",
      borderRadius: "4px",
      background: "rgba(245, 158, 11, 0.2)",
      color: "#fde68a",
    },
    ".cm-md-quote": {
      color: "#aaa",
      boxShadow: "inset 2px 0 0 #222",
      paddingLeft: "12px",
    },
    ".cm-md-bullet": {
      display: "inline-block",
      width: "1.1em",
      color: "#fff",
    },
    ".cm-md-link": {
      color: "#fde68a",
      textDecoration: "underline",
      cursor: "pointer",
    },
    ".cm-md-fence": {
      background: "#000",
      fontFamily: 'ui-monospace, "Geist Mono", "Cascadia Code", Consolas, monospace',
      fontSize: "13px",
    },
  },
  { dark: true },
);
