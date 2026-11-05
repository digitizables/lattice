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
  let lightbox = $state<{ src: string; alt: string } | null>(null);

  const currentSet = $derived(findDocSet(docSetId));
  const currentDoc = $derived(findDoc(docSetId, docId));
  const html = $derived(renderMarkdown(currentDoc.markdown));

  function selectSet(id: DocSetId) {
    docSetId = id;
    localStorage.setItem(DOC_SET_KEY, id);
    const next = findDocSet(id).docs[0]?.id ?? "";
    docId = next;
    localStorage.setItem(DOC_ID_KEY, next);
    lightbox = null;
  }

  function selectDoc(id: string) {
    docId = id;
    localStorage.setItem(DOC_ID_KEY, id);
    lightbox = null;
  }

  function openLightbox(img: HTMLImageElement) {
    lightbox = {
      src: img.currentSrc || img.src,
      alt: img.alt || "Documentation figure",
    };
  }

  function closeLightbox() {
    lightbox = null;
  }

  function onArticleClick(e: MouseEvent) {
    const t = e.target;
    if (!(t instanceof Element)) return;

    if (t instanceof HTMLImageElement) {
      e.preventDefault();
      openLightbox(t);
      return;
    }

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

  $effect(() => {
    if (!lightbox) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        closeLightbox();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });
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

  <div class="docs-split">
    <aside class="docs-rail" aria-label="{currentSet.label} documents">
      <div class="docs-rail-head">
        <h3>{currentSet.label}</h3>
        <p>{currentSet.description}</p>
      </div>
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

    <article class="docs-article">
      <header class="docs-article-head">
        <h3>{currentDoc.title}</h3>
        <p class="mono docs-path">{currentDoc.id}.md</p>
      </header>
      <!-- Trusted first-party markdown bundled at build time.
           Click handler only remaps in-set relative links; real <a> keys still work. -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="docs-prose" role="region" aria-label="Document content" onclick={onArticleClick}>{@html html}</div>
    </article>
  </div>
</div>

{#if lightbox}
  <div
    class="docs-lightbox"
    role="dialog"
    aria-modal="true"
    aria-label={lightbox.alt}
    tabindex="-1"
    onclick={(e) => {
      if (e.currentTarget === e.target) closeLightbox();
    }}
    onkeydown={(e) => {
      if (e.key === "Escape" || e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        closeLightbox();
      }
    }}
  >
    <button type="button" class="docs-lightbox-close ghost-btn" onclick={closeLightbox} aria-label="Close image"
      >Close</button
    >
    <img class="docs-lightbox-img" src={lightbox.src} alt={lightbox.alt} />
  </div>
{/if}
