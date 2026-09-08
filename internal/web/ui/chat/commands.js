/**
 * Slash commands expand into a full prompt before the message is sent.
 * Only a line that starts with `/` is treated as a command.
 */

export const CHAT_COMMANDS = [
  {
    id: "create",
    names: ["new", "create", "создать"],
    title: "Новая заметка",
    hint: "название",
    needsArg: true,
    expand: (arg) => (arg ? `Создай новую заметку ${arg}` : "Создай новую заметку"),
  },
  {
    id: "search",
    names: ["search", "find", "найди"],
    title: "Поиск по заметкам",
    hint: "запрос",
    needsArg: true,
    expand: (arg) => (arg ? `Найди в заметках: ${arg}` : "Найди заметки по моему запросу"),
  },
  {
    id: "read",
    names: ["read", "open", "прочитай"],
    title: "Прочитать заметку",
    hint: "название",
    needsArg: true,
    expand: (arg) => (
      arg
        ? `Прочитай заметку ${arg} и кратко расскажи, о чём она`
        : "Прочитай указанную заметку и кратко расскажи, о чём она"
    ),
  },
  {
    id: "append",
    names: ["append", "add", "допиши"],
    title: "Дописать в заметку",
    hint: "название: текст",
    needsArg: true,
    expand: (arg) => {
      const { note, text } = splitNoteText(arg);
      if (note && text) return `Допиши в заметку ${note} следующее: ${text}`;
      if (note) return `Допиши в заметку ${note}`;
      return "Допиши в указанную заметку";
    },
  },
  {
    id: "rewrite",
    names: ["rewrite", "update", "перепиши"],
    title: "Переписать заметку",
    hint: "название",
    needsArg: true,
    expand: (arg) => (
      arg
        ? `Перепиши заметку ${arg} целиком. Сохрани смысл, улучши ясность и структуру`
        : "Перепиши указанную заметку целиком. Сохрани смысл, улучши ясность и структуру"
    ),
  },
  {
    id: "summarize",
    names: ["summarize", "sum", "суммируй"],
    title: "Краткий пересказ",
    hint: "название",
    needsArg: true,
    expand: (arg) => (
      arg
        ? `Суммируй заметку ${arg} в нескольких предложениях`
        : "Суммируй указанную заметку в нескольких предложениях"
    ),
  },
  {
    id: "list",
    names: ["list", "notes", "список"],
    title: "Список заметок",
    hint: "",
    needsArg: false,
    expand: () => "Покажи список моих заметок, сгруппированных по разделам",
  },
  {
    id: "help",
    names: ["help", "помощь"],
    title: "Что умеет агент",
    hint: "",
    needsArg: false,
    expand: () => "Кратко объясни, что ты умеешь делать с заметками и какие команды тебе доступны",
  },
];

export function splitNoteText(arg) {
  const raw = String(arg || "").trim();
  if (!raw) return { note: "", text: "" };
  const m = raw.match(/^(.+?)\s*[:：]\s*([\s\S]+)$/);
  if (!m) return { note: raw, text: "" };
  return { note: m[1].trim(), text: m[2].trim() };
}

export function parseSlashInput(text) {
  const raw = String(text || "");
  if (!raw.startsWith("/") || raw.startsWith("//")) return null;
  const body = raw.slice(1);
  const space = body.search(/\s/);
  const tokenRaw = space === -1 ? body : body.slice(0, space);
  const token = tokenRaw.toLowerCase();
  const rest = space === -1 ? "" : body.slice(space + 1).replace(/^\s+/, "");
  return { token, tokenRaw, rest, raw };
}

export function matchCommands(parsed) {
  if (!parsed) return [];
  const token = parsed.token;
  if (!token) return CHAT_COMMANDS;
  return CHAT_COMMANDS.filter((cmd) => cmd.names.some((name) => name.startsWith(token)));
}

export function resolveCommand(parsed) {
  if (!parsed?.token) return null;
  return CHAT_COMMANDS.find((cmd) => cmd.names.includes(parsed.token)) || null;
}

export function expandSlash(text) {
  const parsed = parseSlashInput(text);
  const cmd = resolveCommand(parsed);
  if (!cmd) return String(text || "");
  return cmd.expand(parsed.rest.trim());
}

export function commandPreview(cmd, rest) {
  if (!cmd) return "";
  const arg = String(rest || "").trim();
  if (arg) return cmd.expand(arg);
  return cmd.hint || "";
}
