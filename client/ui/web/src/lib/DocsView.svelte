<script lang="ts">
  import {
    docSets,
    findDoc,
    findDocSet,
    resolveDocLink,
    type DocSetId,
  } from "./docsCatalog";
  import { renderMarkdown } from "./markdown";

  const DOC_SET_KEY = "lattice.docs.set";
  const DOC_ID_KEY = "lattice.docs.docId";

  function loadSet(): DocSetId {
    const v = localStorage.getItem(DOC_SET_KEY);
    if (v === "operators" || v === "developers") return v;
    return "operators";
  }

  function loadDocId(setId: DocSetId): string {
    const saved = localStorage.getItem(DOC_ID_KEY) ?? "";
    const set = findDocSet(setId);
    if (set.docs.some((d) => d.id === saved)) return saved;
    return set.docs[0]?.id ?? "";
  }

  let docSetId = $state<DocSetId>(loadSet());
  let docId = $state(loadDocId(loadSet()));

  const currentSet = $derived(findDocSet(docSetId));
  const currentDoc = $derived(findDoc(docSetId, docId));
  const html = $derived(renderMarkdown(currentDoc.markdown));

  function selectSet(id: DocSetId) {
    docSetId = id;
    localStorage.setItem(DOC_SET_KEY, id);
    const next = findDocSet(id).docs[0]?.id ?? "";
    docId = next;
    localStorage.setItem(DOC_ID_KEY, next);
  }

  function selectDoc(id: string) {
    docId = id;
    localStorage.setItem(DOC_ID_KEY, id);
  }

  function onArticleClick(e: MouseEvent) {
    const t = e.target;
    if (!(t instanceof Element)) return;
    const a = t.closest("a");
    if (!a) return;
    const href = a.getAttribute("href");
    if (!href || href.startsWith("http://") || href.startsWith("https://") || href.startsWith("#")) {
      return;
    }
    const linked = resolveDocLink(docSetId, href);
    if (linked) {
      e.preventDefault();
      selectDoc(linked.id);
    }
  }
</script>

<div class="surface docs-surface">
  <div class="heading">
    <div>
      <h2>Documentation</h2>
      <p>Operator and developer guides shipped with Lattice.</p>
    </div>
    <div class="heading-actions">
      <div class="segmented" role="group" aria-label="Documentation set">
        {#each docSets as set}
          <button type="button" class:active={docSetId === set.id} onclick={() => selectSet(set.id)}
            >{set.label}</button
          >
        {/each}
      </div>
    </div>
  </div>

  <div class="pane-body docs-page">
    <aside class="docs-rail settings-card">
      <header class="settings-card-head">
        <div>
          <h3>{currentSet.label}</h3>
          <p>{currentSet.description}</p>
        </div>
      </header>
      <nav class="docs-toc" aria-label="Documents in this set">
        {#each currentSet.docs as doc}
          <button
            type="button"
            class="docs-toc-item"
            class:active={docId === doc.id}
            onclick={() => selectDoc(doc.id)}
          >
            <span class="docs-toc-title">{doc.title}</span>
            <span class="docs-toc-desc">{doc.description}</span>
          </button>
        {/each}
      </nav>
    </aside>

    <article class="docs-article settings-card">
      <header class="settings-card-head docs-article-head">
        <div>
          <h3>{currentDoc.title}</h3>
          <p class="mono docs-path">{currentDoc.id}.md</p>
        </div>
      </header>
      <!-- Trusted first-party markdown bundled at build time.
           Click handler only remaps in-set relative links; real <a> keys still work. -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="docs-prose" role="region" aria-label="Document content" onclick={onArticleClick}>{@html html}</div>
    </article>
  </div>
</div>
