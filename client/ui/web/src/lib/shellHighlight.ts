/** Lightweight shell / PowerShell syntax highlighter for the operator console. */

export type ShellDialect = "cmd" | "powershell" | "bash";

const CMD_BUILTINS = new Set(
  [
    "assoc", "break", "call", "cd", "chdir", "cls", "color", "copy", "date", "del", "dir",
    "echo", "endlocal", "erase", "exit", "for", "ftype", "goto", "if", "md", "mkdir", "move",
    "path", "pause", "popd", "prompt", "pushd", "rd", "rem", "ren", "rename", "rmdir", "set",
    "setlocal", "shift", "start", "time", "title", "type", "ver", "verify", "vol", "where",
    "whoami", "hostname", "pwd", "cmd", "powershell", "pwsh",
  ].map((s) => s.toLowerCase()),
);

const BASH_BUILTINS = new Set(
  [
    "alias", "bg", "bind", "break", "builtin", "caller", "cd", "command", "compgen", "complete",
    "continue", "declare", "dirs", "disown", "echo", "enable", "eval", "exec", "exit", "export",
    "false", "fc", "fg", "getopts", "hash", "history", "jobs", "kill", "let", "local", "logout",
    "ls", "mkdir", "pwd", "read", "readonly", "return", "set", "shift", "shopt", "source",
    "suspend", "test", "times", "trap", "true", "type", "typeset", "ulimit", "umask", "unalias",
    "unset", "wait", "which", "whoami", "hostname", "clear", "cat", "grep", "find", "chmod",
    "chown", "cp", "mv", "rm", "touch", "head", "tail", "less", "more", "bash", "sh", "sudo",
  ].map((s) => s.toLowerCase()),
);

const PS_BUILTINS = new Set(
  [
    "cd", "chdir", "cls", "echo", "pwd", "dir", "ls", "cat", "type", "gi", "gp", "gc", "gm",
    "fl", "ft", "select", "where", "foreach", "sort", "measure", "write-host", "write-output",
    "write-error", "get-childitem", "get-content", "get-process", "get-service", "get-item",
    "get-location", "set-location", "set-content", "remove-item", "new-item", "copy-item",
    "move-item", "rename-item", "invoke-command", "invoke-expression", "invoke-webrequest",
    "invoke-restmethod", "whoami", "hostname", "powershell", "pwsh",
  ].map((s) => s.toLowerCase()),
);

export function dialectForSession(osName?: string, line = ""): ShellDialect {
  const head = line.trim().split(/\s+/)[0]?.toLowerCase() ?? "";
  if (head === "powershell" || head === "pwsh" || head === "ps") return "powershell";
  if (head === "bash" || head === "sh") return "bash";
  if (head === "cmd") return "cmd";
  if ((osName || "").toLowerCase().includes("windows")) return "cmd";
  return "bash";
}

export function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function isBuiltin(word: string, dialect: ShellDialect): boolean {
  const lower = word.toLowerCase();
  if (dialect === "powershell") {
    return PS_BUILTINS.has(lower) || /^[a-z]+-[a-z][a-z0-9]*$/i.test(word);
  }
  if (dialect === "cmd") return CMD_BUILTINS.has(lower);
  return BASH_BUILTINS.has(lower);
}

function isPathish(word: string): boolean {
  return (
    /^[A-Za-z]:[\\/]/.test(word) ||
    word.startsWith("/") ||
    word.startsWith("./") ||
    word.startsWith("../") ||
    word.startsWith("~") ||
    word.includes("\\") ||
    (word.includes("/") && !word.startsWith("-"))
  );
}

/**
 * Highlight a shell command line. Returns HTML with span.syn-* tokens.
 * Escapes all literal text.
 */
export function highlightShell(line: string, dialect: ShellDialect = "bash"): string {
  if (!line) return "";

  let i = 0;
  const n = line.length;
  let out = "";
  let atCommandPosition = true;

  while (i < n) {
    const ch = line[i];

    if (ch === "#" || (dialect === "cmd" && line.startsWith("::", i))) {
      out += `<span class="syn-comment">${escapeHtml(line.slice(i))}</span>`;
      break;
    }

    if (/\s/.test(ch)) {
      let j = i + 1;
      while (j < n && /\s/.test(line[j])) j++;
      out += escapeHtml(line.slice(i, j));
      i = j;
      continue;
    }

    if (ch === "'" || ch === '"') {
      const quote = ch;
      let j = i + 1;
      while (j < n) {
        if (dialect === "powershell" && line[j] === "`" && j + 1 < n) {
          j += 2;
          continue;
        }
        if (dialect !== "cmd" && line[j] === "\\" && j + 1 < n) {
          j += 2;
          continue;
        }
        if (line[j] === quote) {
          j++;
          break;
        }
        j++;
      }
      out += `<span class="syn-string">${escapeHtml(line.slice(i, j))}</span>`;
      i = j;
      atCommandPosition = false;
      continue;
    }

    if ("|&;><".includes(ch)) {
      let j = i + 1;
      if ((ch === "|" || ch === "&" || ch === ">") && j < n && line[j] === ch) j++;
      out += `<span class="syn-operator">${escapeHtml(line.slice(i, j))}</span>`;
      const op = line.slice(i, j);
      i = j;
      atCommandPosition = op === "|" || op === "||" || op === ";" || op === "&" || op === "&&";
      continue;
    }

    if (ch === "$" || (ch === "%" && dialect === "cmd")) {
      let j = i + 1;
      if (ch === "%") {
        while (j < n && line[j] !== "%" && !/\s/.test(line[j])) j++;
        if (j < n && line[j] === "%") j++;
      } else if (j < n && line[j] === "{") {
        j++;
        while (j < n && line[j] !== "}") j++;
        if (j < n) j++;
      } else {
        while (j < n && /[A-Za-z0-9_:?-]/.test(line[j])) j++;
      }
      out += `<span class="syn-variable">${escapeHtml(line.slice(i, j))}</span>`;
      i = j;
      atCommandPosition = false;
      continue;
    }

    if (
      (ch === "-" && i + 1 < n && /[A-Za-z?]/.test(line[i + 1])) ||
      (ch === "/" && dialect === "cmd" && i + 1 < n && /[A-Za-z?]/.test(line[i + 1]))
    ) {
      let j = i + 1;
      while (j < n && /[A-Za-z0-9_?:-]/.test(line[j])) j++;
      out += `<span class="syn-flag">${escapeHtml(line.slice(i, j))}</span>`;
      i = j;
      atCommandPosition = false;
      continue;
    }

    if (/\d/.test(ch) && !atCommandPosition) {
      let j = i + 1;
      while (j < n && /[\d.]/.test(line[j])) j++;
      out += `<span class="syn-number">${escapeHtml(line.slice(i, j))}</span>`;
      i = j;
      continue;
    }

    let j = i;
    while (j < n) {
      const c = line[j];
      if (/[\s|&;><`'"]/.test(c)) break;
      if (c === "#" && dialect !== "cmd") break;
      if (c === "$") break;
      if (c === "%" && dialect === "cmd") break;
      j++;
    }

    const word = line.slice(i, j);
    if (atCommandPosition && isBuiltin(word, dialect)) {
      out += `<span class="syn-command">${escapeHtml(word)}</span>`;
    } else if (atCommandPosition) {
      out += `<span class="syn-command">${escapeHtml(word)}</span>`;
    } else if (isPathish(word)) {
      out += `<span class="syn-path">${escapeHtml(word)}</span>`;
    } else {
      out += escapeHtml(word);
    }
    atCommandPosition = false;
    i = j;
  }

  return out;
}

/** Escape plain command output for HTML; CSS pre-wrap keeps newlines and wraps long lines. */
export function formatTermOutput(text: string): string {
  return escapeHtml(text.replace(/\r\n/g, "\n").replace(/\r/g, "\n"));
}
