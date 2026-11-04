<script lang="ts">
  import { tick } from "svelte";
  import {
    accountName,
    formatPromptPath,
    listSessions,
    submitTaskAndWait,
    userAtHost,
    type PathDisplay,
    type Session,
  } from "./api";
  import { formatTermOutput } from "./ansiOutput";
  import { dialectForSession, escapeHtml, highlightShell } from "./shellHighlight";

  let {
    agentId = "",
    pathDisplay,
    pickerSessions = [],
    pickerQuery = "",
    active = false,
    showClose = false,
    title = "Terminal",
    onFocus = undefined as undefined | (() => void),
    onClose = undefined as undefined | (() => void),
    onPickerQuery = undefined as undefined | ((q: string) => void),
    onPickerSearch = undefined as undefined | (() => void),
    onAgentChange = undefined as undefined | ((id: string) => void),
    onSessionUpdated = undefined as undefined | ((s: Session) => void),
    onContextMenu = undefined as undefined | ((x: number, y: number) => void),
  }: {
    agentId?: string;
    pathDisplay: PathDisplay;
    pickerSessions?: Session[];
    pickerQuery?: string;
    active?: boolean;
    showClose?: boolean;
    title?: string;
    onFocus?: () => void;
    onClose?: () => void;
    onPickerQuery?: (q: string) => void;
    onPickerSearch?: () => void;
    onAgentChange?: (id: string) => void;
    onSessionUpdated?: (s: Session) => void;
    onContextMenu?: (x: number, y: number) => void;
  } = $props();

  let lines = $state<string[]>([]);
  let input = $state("");
  let history = $state<string[]>([]);
  let historyIdx = $state(-1);
  let busy = $state(false);
  let termEl: HTMLDivElement | undefined = $state();
  let inputEl: HTMLInputElement | undefined = $state();
  let activeAbort: AbortController | null = null;
  /** Pointer origin for drag-select; used so mouseup does not steal focus. */
  let pointerDown: { x: number; y: number } | null = null;

  const session = $derived(
    pickerSessions.find((s) => s.agentId === agentId) ?? null,
  );

  const highlightedInput = $derived(
    highlightShell(input, dialectForSession(session?.os, input)),
  );

  function guessDefaultCwd(osName?: string): string {
    if ((osName || "").toLowerCase().includes("windows")) return "C:\\";
    return "/";
  }

  function promptPrefix(agent?: Session | null) {
    if (!agent) return `<span class="term-prompt">lattice&gt;</span>`;
    const user = accountName(agent.username, agent.hostname);
    const path = formatPromptPath(agent.cwd || guessDefaultCwd(agent.os), pathDisplay);
    const pathHtml = path ? `:<span class="term-path">${escapeHtml(path)}</span>` : "";
    return `<span class="term-identity">${escapeHtml(user)}</span>@<span class="term-path">${escapeHtml(agent.hostname)}</span>${pathHtml} <span class="term-prompt">$</span>`;
  }

  function isOnline(s: Session, now = Date.now()): boolean {
    if (!s.lastSeen) return false;
    const t = Date.parse(s.lastSeen);
    if (Number.isNaN(t)) return false;
    return now - t < 30_000;
  }

  function hasCanvasSelection(): boolean {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || !sel.rangeCount) return false;
    const node = sel.anchorNode;
    if (!node || !termEl) return false;
    return termEl.contains(node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement);
  }

  async function appendTerm(html: string) {
    const keepSelection = hasCanvasSelection();
    lines = [...lines, html];
    await tick();
    // Do not jump the viewport while the operator is selecting text to copy.
    if (termEl && !keepSelection) termEl.scrollTop = termEl.scrollHeight;
  }

  export function clearScrollback() {
    lines = [];
  }

  export function focusInput() {
    if (hasCanvasSelection()) return;
    void tick().then(() => inputEl?.focus());
  }

  export function interrupt() {
    if (!busy || !activeAbort) return;
    activeAbort.abort();
  }

  let wasActive = false;
  $effect(() => {
    if (active && !wasActive) focusInput();
    wasActive = active;
  });

  $effect(() => {
    if (!busy) return;
    const onKey = (e: KeyboardEvent) => {
      if (!active) return;
      if (e.key === "c" && (e.ctrlKey || e.metaKey)) {
        // Prefer copy when scrollback text is selected.
        if (hasCanvasSelection()) return;
        e.preventDefault();
        interrupt();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  });

  async function runShellLine(line: string) {
    if (!agentId) {
      await appendTerm('<span class="term-err">no session selected — pick one above</span>');
      return;
    }
    if (busy) {
      await appendTerm('<span class="term-err">^C to cancel the running command</span>');
      return;
    }

    busy = true;
    const agent = session;
    if (agent && !isOnline(agent)) {
      await appendTerm('<span class="term-err">session offline (no recent check-in)</span>');
    }
    await appendTerm(
      `${promptPrefix(agent)} ${highlightShell(line, dialectForSession(agent?.os, line))}`,
    );
    const ac = new AbortController();
    activeAbort = ac;
    let shownOut = 0;
    let shownErr = 0;
    const paintLive = async (stdout: string, stderr: string) => {
      const out = stdout.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
      const err = stderr.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
      if (out.length > shownOut) {
        const chunk = out.slice(shownOut);
        shownOut = out.length;
        if (chunk.trimEnd() !== "" || chunk.includes("\n")) {
          await appendTerm(formatTermOutput(chunk.replace(/\n$/, "")));
        }
      }
      if (err.length > shownErr) {
        const chunk = err.slice(shownErr);
        shownErr = err.length;
        if (chunk.trimEnd() !== "" || chunk.includes("\n")) {
          await appendTerm(
            `<span class="term-err">${formatTermOutput(chunk.replace(/\n$/, ""))}</span>`,
          );
        }
      }
    };
    try {
      const result = await submitTaskAndWait(
        agentId,
        "shell",
        [line],
        undefined,
        undefined,
        ac.signal,
        (stdout, stderr) => {
          void paintLive(stdout, stderr);
        },
      );
      await paintLive(result.stdout || "", result.stderr || "");
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg === "^C") {
        await appendTerm('<span class="term-err">^C</span>');
      } else {
        await appendTerm(`<span class="term-err">${escapeHtml(msg)}</span>`);
      }
    } finally {
      busy = false;
      activeAbort = null;
      focusInput();
    }
  }

  async function handleCommand(raw: string) {
    const trimmed = raw.trim();
    if (!trimmed) return;
    history = [trimmed, ...history.filter((h) => h !== trimmed)].slice(0, 50);
    historyIdx = -1;
    await runShellLine(trimmed);
    if (agentId) {
      try {
        const res = await listSessions({ pageSize: 5, query: agentId });
        const updated = res.sessions.find((s) => s.agentId === agentId);
        if (updated) onSessionUpdated?.(updated);
      } catch {
        // ignore refresh errors
      }
    }
  }

  async function onKeydown(e: KeyboardEvent) {
    onFocus?.();
    if (e.key === "c" && (e.ctrlKey || e.metaKey)) {
      if (hasCanvasSelection()) return;
      if (busy) {
        e.preventDefault();
        interrupt();
      }
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      if (busy) return;
      const value = input;
      input = "";
      await handleCommand(value);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (history.length === 0) return;
      historyIdx = Math.min(historyIdx + 1, history.length - 1);
      input = history[historyIdx] ?? "";
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (historyIdx <= 0) {
        historyIdx = -1;
        input = "";
        return;
      }
      historyIdx -= 1;
      input = history[historyIdx] ?? "";
      return;
    }
    if (e.key === "l" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      if (!busy) clearScrollback();
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="term-pane"
  class:active
  role="group"
  aria-label={title}
  onclick={() => onFocus?.()}
  oncontextmenu={(e) => {
    e.preventDefault();
    e.stopPropagation();
    onFocus?.();
    onContextMenu?.(e.clientX, e.clientY);
  }}
>
  <div class="term-pane-bar">
    <div class="term-pane-title">{title}</div>
    <div class="agent-picker compact">
      <input
        class="search"
        placeholder="Filter…"
        value={pickerQuery}
        oninput={(e) => {
          onPickerQuery?.((e.currentTarget as HTMLInputElement).value);
          onPickerSearch?.();
        }}
        onclick={(e) => e.stopPropagation()}
      />
      <select
        value={agentId}
        onclick={(e) => e.stopPropagation()}
        onchange={(e) => {
          const next = (e.currentTarget as HTMLSelectElement).value;
          onAgentChange?.(next);
          clearScrollback();
          focusInput();
          onFocus?.();
        }}
      >
        <option value="">Select session…</option>
        {#each pickerSessions as s}
          <option value={s.agentId}
            >{isOnline(s) ? "●" : "○"} {userAtHost(s.username, s.hostname)} · {s.agentId.slice(0, 8)}</option
          >
        {/each}
      </select>
    </div>
    <div class="term-pane-actions">
      <button
        type="button"
        class="ghost-btn"
        title="Clear scrollback (Ctrl+L)"
        onclick={(e) => {
          e.stopPropagation();
          clearScrollback();
        }}>Clear</button
      >
      {#if showClose}
        <button
          type="button"
          class="ghost-btn icon-btn"
          title="Close pane"
          aria-label="Close pane"
          onclick={(e) => {
            e.stopPropagation();
            onClose?.();
          }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
            ><path d="M18 6 6 18M6 6l12 12" /></svg
          >
        </button>
      {/if}
    </div>
  </div>

  <div class="terminal-surface fill">
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="terminal-canvas mono"
      role="log"
      tabindex="0"
      bind:this={termEl}
      onpointerdown={(e) => {
        if (e.button === 0) pointerDown = { x: e.clientX, y: e.clientY };
      }}
      onclick={(e) => {
        onFocus?.();
        const dragged =
          pointerDown != null &&
          (Math.abs(e.clientX - pointerDown.x) > 4 || Math.abs(e.clientY - pointerDown.y) > 4);
        pointerDown = null;
        // Keep scrollback selection for copy; only plain clicks focus the prompt.
        if (dragged || hasCanvasSelection()) return;
        focusInput();
      }}
    >
      {#each lines as line, i (i)}
        <div class="term-line">{@html line}</div>
      {/each}
    </div>
    <div class="term-input-row mono">
      <span class="prompt">{@html promptPrefix(session)}</span>
      <div class="term-input-wrap">
        <div class="term-input-mirror" aria-hidden="true">
          {#if input}
            {@html highlightedInput}
          {:else if !busy}
            <span class="term-placeholder">command</span>
          {:else}
            <span class="term-placeholder">waiting — Ctrl+C to cancel</span>
          {/if}
        </div>
        <input
          class="term-input"
          bind:this={inputEl}
          bind:value={input}
          readonly={busy}
          spellcheck="false"
          autocomplete="off"
          autocapitalize="off"
          aria-label="Shell command"
          onkeydown={onKeydown}
          onfocus={() => onFocus?.()}
        />
      </div>
    </div>
  </div>
</div>
