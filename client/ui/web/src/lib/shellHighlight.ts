/**
 * Command-line highlighting via highlight.js (bash / PowerShell / DOS batch).
 * Output coloring is handled separately by ansiOutput.ts (ansi_up).
 */
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import dos from "highlight.js/lib/languages/dos";
import powershell from "highlight.js/lib/languages/powershell";

hljs.registerLanguage("bash", bash);
hljs.registerLanguage("powershell", powershell);
hljs.registerLanguage("dos", dos);

export type ShellDialect = "cmd" | "powershell" | "bash";

export function dialectForSession(osName?: string, line = ""): ShellDialect {
  const head = line.trim().split(/\s+/)[0]?.toLowerCase() ?? "";
  if (head === "powershell" || head === "pwsh" || head === "ps") return "powershell";
  if (head === "bash" || head === "sh") return "bash";
  if (head === "cmd") return "cmd";
  if ((osName || "").toLowerCase().includes("windows")) return "cmd";
  return "bash";
}

function hljsLang(dialect: ShellDialect): string {
  switch (dialect) {
    case "powershell":
      return "powershell";
    case "cmd":
      return "dos";
    default:
      return "bash";
  }
}

export function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** Highlight a shell command line with highlight.js. Falls back to escaped text. */
export function highlightShell(line: string, dialect: ShellDialect = "bash"): string {
  if (!line) return "";
  try {
    return hljs.highlight(line, { language: hljsLang(dialect), ignoreIllegals: true }).value;
  } catch {
    return escapeHtml(line);
  }
}
