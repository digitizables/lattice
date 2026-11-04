/**
 * Terminal output rendering via ansi_up — full ANSI SGR / 256-color coverage.
 * Used for agent stdout/stderr (ls colors, grep --color, etc.).
 */
import { AnsiUp } from "ansi_up";

const ansi = new AnsiUp();
ansi.use_classes = false;
ansi.escape_html = true;

/** Convert agent output (possibly with ANSI escapes) into safe colored HTML. */
export function formatTermOutput(text: string): string {
  const normalized = text.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  return ansi.ansi_to_html(normalized);
}
