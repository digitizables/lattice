<script lang="ts">
  import { tick } from "svelte";
  import {
    accountName,
    deleteSession,
    formatPromptPath,
    listSessions,
    loadPathDisplay,
    savePathDisplay,
    submitTaskAndWait,
    userAtHost,
    type PathDisplay,
    type Session,
  } from "./lib/api";
  import DocsView from "./lib/DocsView.svelte";
  import {
    dialectForSession,
    escapeHtml,
    formatTermOutput,
    highlightShell,
  } from "./lib/shellHighlight";

  type NavId = "sessions" | "console" | "listeners" | "docs" | "settings";

  const pageSize = 50;

  let sessions = $state<Session[]>([]);
  let totalCount = $state(0);
  let activeCount = $state(0);
  let nextPageToken = $state("");
  let pageTokens = $state<string[]>([""]); // tokens to fetch each page; index 0 is first page
  let pageIndex = $state(0);
  let query = $state("");
  let queryInput = $state("");
  let error = $state("");
  let listLoading = $state(false);
  let taskBusy = $state(false);
  let nav = $state<NavId>("sessions");
  let selectedAgent = $state("");
  let selectedSession = $state<Session | null>(null);
  let termLines = $state<string[]>([]);
  let consoleBooted = $state(false);
  let sidebarExpanded = $state(false);
  let pathDisplay = $state<PathDisplay>(loadPathDisplay());
  let termInput = $state("");
  let history = $state<string[]>([]);
  let historyIdx = $state(-1);
  let termEl: HTMLDivElement | undefined = $state();
  let inputEl: HTMLInputElement | undefined = $state();
  let pickerQuery = $state("");
  let pickerSessions = $state<Session[]>([]);
  let searchTimer: ReturnType<typeof setTimeout> | undefined;
  let activeAbort: AbortController | null = null;
  let activeTaskId = $state("");

  const selected = $derived(
    selectedSession && selectedSession.agentId === selectedAgent
      ? selectedSession
      : sessions.find((s) => s.agentId === selectedAgent) ??
          pickerSessions.find((s) => s.agentId === selectedAgent) ??
          null,
  );

  function promptPrefix(agent?: Session | null) {
    if (!agent) return `<span class="term-prompt">lattice&gt;</span>`;
    const user = accountName(agent.username, agent.hostname);
    const path = formatPromptPath(agent.cwd || guessDefaultCwd(agent.os), pathDisplay);
    const pathHtml = path
      ? `:<span class="term-path">${escapeHtml(path)}</span>`
      : "";
    return `<span class="term-identity">${escapeHtml(user)}</span>@<span class="term-path">${escapeHtml(agent.hostname)}</span>${pathHtml} <span class="term-prompt">$</span>`;
  }

  function guessDefaultCwd(osName?: string): string {
    if ((osName || "").toLowerCase().includes("windows")) return "C:\\";
    return "/";
  }

  function clearScrollback() {
    termLines = [];
    consoleBooted = true;
  }

  function bootConsoleBanner(_agent?: Session | null) {
    clearScrollback();
  }

  function highlightLine(line: string, agent?: Session | null): string {
    return highlightShell(line, dialectForSession(agent?.os, line));
  }

  const highlightedInput = $derived(
    highlightShell(termInput, dialectForSession(selected?.os, termInput)),
  );

  async function appendTerm(html: string) {
    termLines = [...termLines, html];
    await tick();
    if (termEl) termEl.scrollTop = termEl.scrollHeight;
  }

  function focusTerminal() {
    void tick().then(() => inputEl?.focus());
  }

  function isOnline(s: Session, now = Date.now()): boolean {
    if (!s.lastSeen) return false;
    const t = Date.parse(s.lastSeen);
    if (Number.isNaN(t)) return false;
    return now - t < 30_000;
  }

  function selectSession(s: Session) {
    selectedAgent = s.agentId;
    selectedSession = s;
  }

  function openConsole(agentId?: string, session?: Session) {
    if (session) selectSession(session);
    else if (agentId) selectedAgent = agentId;
    nav = "console";
    if (!consoleBooted) {
      clearScrollback();
    }
    focusTerminal();
    void loadPicker("");
  }

  function setPathDisplay(mode: PathDisplay) {
    pathDisplay = mode;
    savePathDisplay(mode);
  }

  async function loadPage(index: number) {
    listLoading = true;
    error = "";
    try {
      const token = pageTokens[index] ?? "";
      const res = await listSessions({
        pageSize,
        pageToken: token || undefined,
        query: query || undefined,
      });
      sessions = res.sessions;
      totalCount = res.totalCount;
      activeCount = res.activeCount;
      nextPageToken = res.nextPageToken;
      pageIndex = index;
      if (res.nextPageToken) {
        pageTokens = [...pageTokens.slice(0, index + 1), res.nextPageToken];
      } else {
        pageTokens = pageTokens.slice(0, index + 1);
      }
      const fresh = sessions.find((s) => isOnline(s));
      if (!selectedAgent) {
        if (fresh) selectSession(fresh);
        else if (sessions.length > 0) selectSession(sessions[0]);
      } else {
        const onPage = sessions.find((s) => s.agentId === selectedAgent);
        if (onPage) selectedSession = onPage;
        // If the selected session went stale and a live one exists, prefer live.
        if (selectedSession && !isOnline(selectedSession) && fresh) {
          selectSession(fresh);
        }
      }
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      listLoading = false;
    }
  }

  function applySearch() {
    query = queryInput.trim();
    pageTokens = [""];
    pageIndex = 0;
    void loadPage(0);
  }

  function scheduleSearch() {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => applySearch(), 250);
  }

  async function loadPicker(q: string) {
    try {
      const res = await listSessions({ pageSize: 30, query: q || undefined });
      pickerSessions = res.sessions;
      if (selectedSession && !pickerSessions.some((s) => s.agentId === selectedSession?.agentId)) {
        pickerSessions = [selectedSession, ...pickerSessions];
      }
    } catch {
      // keep prior picker list
    }
  }

  function schedulePickerSearch() {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => void loadPicker(pickerQuery.trim()), 250);
  }

  async function removeSession(agentId: string) {
    error = "";
    try {
      await deleteSession(agentId);
      if (selectedAgent === agentId) {
        selectedAgent = "";
        selectedSession = null;
      }
      await loadPage(pageIndex);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function runShellLine(line: string) {
    if (!selectedAgent) {
      await appendTerm('<span class="term-err">no session selected — pick one in the console toolbar</span>');
      return;
    }
    if (taskBusy) {
      await appendTerm('<span class="term-err">^C to cancel the running command</span>');
      return;
    }

    taskBusy = true;
    error = "";
    const agent = selected;
    if (agent && !isOnline(agent)) {
      await appendTerm(
        '<span class="term-err">session offline (no recent check-in)</span>',
      );
    }
    await appendTerm(`${promptPrefix(agent)} ${highlightLine(line, agent)}`);
    const ac = new AbortController();
    activeAbort = ac;
    activeTaskId = "";
    try {
      const result = await submitTaskAndWait(
        selectedAgent,
        "shell",
        [line],
        undefined,
        (taskId) => {
          activeTaskId = taskId;
        },
        ac.signal,
      );
      if (result.stdout) {
        await appendTerm(formatTermOutput(result.stdout.replace(/\n$/, "")));
      }
      if (result.stderr) {
        await appendTerm(
          `<span class="term-err">${formatTermOutput(result.stderr.replace(/\n$/, ""))}</span>`,
        );
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg === "^C") {
        await appendTerm('<span class="term-err">^C</span>');
      } else {
        await appendTerm(`<span class="term-err">${escapeHtml(msg)}</span>`);
      }
    } finally {
      taskBusy = false;
      activeAbort = null;
      activeTaskId = "";
      focusTerminal();
    }
  }

  async function handleCommand(raw: string) {
    const trimmed = raw.trim();
    if (!trimmed) return;

    history = [trimmed, ...history.filter((h) => h !== trimmed)].slice(0, 50);
    historyIdx = -1;

    // Pure terminal emulator: every line goes to the agent's native OS shell.
    await runShellLine(trimmed);
    if (selectedAgent) {
      const res = await listSessions({ pageSize: 5, query: selectedAgent });
      const updated = res.sessions.find((s) => s.agentId === selectedAgent);
      if (updated) selectSession(updated);
    }
  }

  function interruptTask() {
    if (!taskBusy || !activeAbort) return;
    activeAbort.abort();
    void appendTerm('<span class="term-muted">^C — cancelled</span>');
  }

  async function onTermKeydown(e: KeyboardEvent) {
    if (e.key === "c" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      interruptTask();
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      if (taskBusy) return;
      const value = termInput;
      termInput = "";
      await handleCommand(value);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (history.length === 0) return;
      historyIdx = Math.min(historyIdx + 1, history.length - 1);
      termInput = history[historyIdx] ?? "";
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (historyIdx <= 0) {
        historyIdx = -1;
        termInput = "";
        return;
      }
      historyIdx -= 1;
      termInput = history[historyIdx] ?? "";
      return;
    }
    if (e.key === "l" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      if (!taskBusy) bootConsoleBanner(selected);
    }
  }

  // Ctrl+C must work even while the input would otherwise ignore keys.
  $effect(() => {
    if (!taskBusy) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "c" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        interruptTask();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  });

  $effect(() => {
    void loadPage(0);
  });

  $effect(() => {
    if (nav !== "sessions") return;
    const idx = pageIndex;
    const id = setInterval(() => {
      void loadPage(idx);
    }, 15000);
    return () => clearInterval(id);
  });

  $effect(() => {
    if (nav !== "console") return;
    if (!consoleBooted) {
      clearScrollback();
    }
    focusTerminal();
    void loadPicker(pickerQuery.trim());
  });
</script>

<div class="app-window">
  <header class="titlebar">
    <div class="title-block">
      <div class="brand">Lattice</div>
      <div class="descriptor">Operator console</div>
    </div>
  </header>

  <div class="shell">
    <aside class="sidebar" class:expanded={sidebarExpanded}>
      <div class="sidebar-top">
        <button
          class="nav-item"
          title={sidebarExpanded ? "Collapse" : "Expand"}
          aria-label="Toggle navigation"
          onclick={() => (sidebarExpanded = !sidebarExpanded)}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            {#if sidebarExpanded}
              <path d="M15 18l-6-6 6-6" />
            {:else}
              <path d="M9 18l6-6-6-6" />
            {/if}
          </svg>
        </button>
      </div>
      <nav class="nav-group">
        <button class="nav-item" class:active={nav === "sessions"} title="Sessions" onclick={() => (nav = "sessions")}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8M12 17v4" /></svg>
          <span class="nav-label">Sessions</span>
        </button>
        <button class="nav-item" class:active={nav === "console"} title="Console" onclick={() => openConsole()}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg>
          <span class="nav-label">Console</span>
        </button>
        <button class="nav-item" class:active={nav === "listeners"} title="Listeners" onclick={() => (nav = "listeners")}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12.55a11 11 0 0 1 14.08 0" /><path d="M1.42 9a16 16 0 0 1 21.16 0" /><path d="M8.53 16.11a6 6 0 0 1 6.95 0" /><circle cx="12" cy="20" r="1" /></svg>
          <span class="nav-label">Listeners</span>
        </button>
      </nav>
      <div class="sidebar-spacer"></div>
      <button class="nav-item" class:active={nav === "docs"} title="Documentation" onclick={() => (nav = "docs")}>
        <!-- Lucide "book-open": https://lucide.dev/icons/book-open -->
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M12 7v14" />
          <path
            d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z"
          />
        </svg>
        <span class="nav-label">Docs</span>
      </button>
      <button class="nav-item" class:active={nav === "settings"} title="Settings" onclick={() => (nav = "settings")}>
        <!-- Lucide "settings" gear: https://lucide.dev/icons/settings -->
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path
            d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"
          />
          <circle cx="12" cy="12" r="3" />
        </svg>
        <span class="nav-label">Settings</span>
      </button>
    </aside>

    <section class="workspace">
      <div class="workspace-body">
        {#if nav === "sessions"}
          <div class="surface">
            <div class="heading">
              <div>
                <h2>Sessions</h2>
                <p>{totalCount.toLocaleString()} agent{totalCount === 1 ? "" : "s"} · page {pageIndex + 1}</p>
              </div>
              <div class="heading-actions">
                <input
                  class="search"
                  placeholder="Search id, host, user, os…"
                  bind:value={queryInput}
                  oninput={() => scheduleSearch()}
                  onkeydown={(e) => {
                    if (e.key === "Enter") applySearch();
                  }}
                />
                <button class="ghost-btn" onclick={() => loadPage(pageIndex)} disabled={listLoading}>Refresh</button>
              </div>
            </div>
            <div class="pane-body grow">
              {#if sessions.length === 0}
                <div class="empty">
                  <strong>{query ? "No matches" : "No sessions yet"}</strong>
                  <span
                    >{#if query}Try a different query.{:else}Start an agent against <span class="mono">https://127.0.0.1:8443</span>.{/if}</span
                  >
                </div>
              {:else}
                <table class="sessions">
                  <thead>
                    <tr>
                      <th>Agent</th>
                      <th>User@Host</th>
                      <th>Status</th>
                      <th>Platform</th>
                      <th>Last seen</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each sessions as s}
                      <tr
                        class:selected={selectedAgent === s.agentId}
                        onclick={() => selectSession(s)}
                        ondblclick={() => openConsole(s.agentId, s)}
                      >
                        <td class="mono" title={s.agentId}>{s.agentId.slice(0, 8)}…</td>
                        <td>{userAtHost(s.username, s.hostname)}</td>
                        <td class={isOnline(s) ? "status-on" : "status-off"}>{isOnline(s) ? "online" : "offline"}</td>
                        <td>{s.os}/{s.arch}</td>
                        <td class="mono">{s.lastSeen ?? "—"}</td>
                        <td class="row-actions">
                          <button
                            class="ghost-btn"
                            onclick={(e) => {
                              e.stopPropagation();
                              openConsole(s.agentId, s);
                            }}>Console</button
                          >
                          <button
                            class="ghost-btn"
                            onclick={(e) => {
                              e.stopPropagation();
                              void removeSession(s.agentId);
                            }}>Remove</button
                          >
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            </div>
            <div class="pager">
              <button class="ghost-btn" disabled={pageIndex === 0 || listLoading} onclick={() => loadPage(pageIndex - 1)}
                >Previous</button
              >
              <span class="muted"
                >{sessions.length ? pageIndex * pageSize + 1 : 0}–{pageIndex * pageSize + sessions.length} of {totalCount.toLocaleString()}</span
              >
              <button
                class="ghost-btn"
                disabled={!nextPageToken || listLoading}
                onclick={() => loadPage(pageIndex + 1)}>Next</button
              >
            </div>
          </div>
        {:else if nav === "console"}
          <div class="surface console-surface">
            <div class="console-toolbar">
              <div class="agent-picker">
                <label for="picker-search">Session</label>
                <input
                  id="picker-search"
                  class="search"
                  placeholder="Filter sessions…"
                  bind:value={pickerQuery}
                  oninput={() => schedulePickerSearch()}
                />
                <select
                  bind:value={selectedAgent}
                  onchange={() => {
                    const s = pickerSessions.find((x) => x.agentId === selectedAgent);
                    if (s) {
                      selectSession(s);
                      bootConsoleBanner(s);
                    }
                    focusTerminal();
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
              <button class="ghost-btn" onclick={() => clearScrollback()} title="Clear scrollback (Ctrl+L)"
                >Clear</button
              >
            </div>
            <div class="terminal-surface fill">
              <div
                class="terminal-canvas mono"
                role="log"
                tabindex="0"
                bind:this={termEl}
                onclick={() => focusTerminal()}
                onkeydown={(e) => {
                  if (e.key.length === 1 || e.key === "Enter" || e.key === "Backspace") focusTerminal();
                }}
              >
                {#each termLines as line}
                  <div class="term-line">{@html line}</div>
                {/each}
              </div>
              <div class="term-input-row mono">
                <span class="prompt">{@html promptPrefix(selected)}</span>
                <div class="term-input-wrap">
                  <div class="term-input-mirror" aria-hidden="true">
                    {#if termInput}
                      {@html highlightedInput}
                    {:else if !taskBusy}
                      <span class="term-placeholder">command</span>
                    {:else}
                      <span class="term-placeholder">waiting — Ctrl+C to cancel</span>
                    {/if}
                  </div>
                  <input
                    class="term-input"
                    bind:this={inputEl}
                    bind:value={termInput}
                    readonly={taskBusy}
                    spellcheck="false"
                    autocomplete="off"
                    autocapitalize="off"
                    aria-label="Shell command"
                    onkeydown={onTermKeydown}
                  />
                </div>
              </div>
            </div>
          </div>
        {:else if nav === "listeners"}
          <div class="surface">
            <div class="heading">
              <div>
                <h2>Listeners</h2>
                <p>Channels started by the teamserver extension loader.</p>
              </div>
            </div>
            <div class="pane-body">
              <table class="sessions">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Kind</th>
                    <th>Bind</th>
                    <th>TLS</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td class="mono">http</td>
                    <td>channel</td>
                    <td class="mono">:8443</td>
                    <td>on</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        {:else if nav === "docs"}
          <DocsView />
        {:else}
          <div class="surface">
            <div class="heading">
              <div>
                <h2>Settings</h2>
                <p>Preferences for this browser profile. Changes apply immediately.</p>
              </div>
            </div>
            <div class="pane-body settings-page">
              <section class="settings-card">
                <header class="settings-card-head">
                  <div>
                    <h3>Terminal</h3>
                    <p>Prompt appearance for the operator console.</p>
                  </div>
                </header>

                <div class="settings-field">
                  <div class="settings-field-label">
                    <span class="settings-field-title">Path in prompt</span>
                    <span class="settings-field-desc"
                      >Choose whether the working directory shows in full or as the last segment.</span
                    >
                  </div>
                  <div class="segmented" role="group" aria-label="Path in prompt">
                    <button
                      type="button"
                      class:active={pathDisplay === "full"}
                      onclick={() => setPathDisplay("full")}>Full</button
                    >
                    <button
                      type="button"
                      class:active={pathDisplay === "partial"}
                      onclick={() => setPathDisplay("partial")}>Partial</button
                    >
                  </div>
                </div>

                <div class="settings-preview">
                  <div class="settings-preview-label">Prompt preview</div>
                  <div class="settings-preview-box mono">
                    <span class="term-identity"
                      >{accountName(selected?.username || "user", selected?.hostname || "host")}</span
                    >@<span class="term-path">{selected?.hostname || "host"}</span>:<span class="term-path"
                      >{formatPromptPath(selected?.cwd || guessDefaultCwd(selected?.os), pathDisplay) ||
                        guessDefaultCwd(selected?.os)}</span
                    >
                    <span class="term-prompt"> $</span>
                  </div>
                  <div class="settings-preview-meta">
                    {#if pathDisplay === "full"}
                      Example: <span class="mono">C:\Users\Paul Beck</span>
                    {:else}
                      Example: <span class="mono">Paul Beck</span> (last path segment)
                    {/if}
                  </div>
                </div>
              </section>

              <section class="settings-card">
                <header class="settings-card-head">
                  <div>
                    <h3>Connection</h3>
                    <p>Where this UI talks to Lattice. Read-only in the lab build.</p>
                  </div>
                </header>
                <div class="settings-kv">
                  <div>
                    <span class="settings-k">Operator API</span>
                    <span class="settings-v mono">http://127.0.0.1:8080</span>
                  </div>
                  <div>
                    <span class="settings-k">Agent channel</span>
                    <span class="settings-v mono">https://127.0.0.1:8443</span>
                  </div>
                </div>
              </section>
            </div>
          </div>
        {/if}
      </div>

      <footer class="footer-bar">
        <span
          ><span class="live">●</span>
          {activeCount.toLocaleString()} active session{activeCount === 1 ? "" : "s"}</span
        >
        <span>↑↓ history · Ctrl+L clear scrollback · Ctrl+C cancel</span>
      </footer>
    </section>
  </div>

  {#if error}
    <p class="error-banner">{error}</p>
  {/if}
</div>
