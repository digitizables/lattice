import { marked } from "marked";

marked.setOptions({
  gfm: true,
  breaks: false,
});

/** Render first-party markdown to HTML (trusted docs bundled at build time). */
export function renderMarkdown(source: string): string {
  return marked.parse(source, { async: false }) as string;
}
