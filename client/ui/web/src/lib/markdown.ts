import { marked } from "marked";

marked.setOptions({
  gfm: true,
  breaks: false,
});

/** Render first-party markdown to HTML (trusted docs bundled at build time). */
export function renderMarkdown(source: string): string {
  // Repo markdown uses relative images/; the SPA serves copies from /docs/developers/images/.
  const rewritten = source.replace(/\]\(images\//g, "](/docs/developers/images/");
  return marked.parse(rewritten, { async: false }) as string;
}
