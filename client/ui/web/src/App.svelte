<script lang="ts">
  import { tick } from "svelte";
  import {
    accountName,
    deleteSession,
    formatPromptPath,
    listSessions,
    loadPathDisplay,
    mayProduceLargeOutput,
    savePathDisplay,
    submitTaskAndWait,
    userAtHost,
    type PathDisplay,
    type Session,
  } from "./lib/api";

  type NavId = "sessions" | "console" | "listeners" | "settings";

  const pageSize = 50;

  let sessions = $state<Session[]>([]);
  let totalCount = $state(0);
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

  function escapeHtml(s: string) {
    return s
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

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

  function bootConsoleBanner(agent?: Session | null) {
    const cwd = agent?.cwd || guessDefaultCwd(agent?.os);
    const shown = formatPromptPath(cwd, pathDisplay) || cwd;
    termLines = [
      `<span class="term-muted">cwd ${escapeHtml(shown)}</span>`,
      '<span class="term-muted">Type a command. Try: help · pwd · cd · dir</span>',
    ];
    consoleBooted = true;
  }

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
    if (!consoleBooted || termLines.length === 0) {
      bootConsoleBanner(session ?? selected);
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

  async function runRemote(cmd: string, args: string[]) {
    if (!selectedAgent) {
      await appendTerm(
        '<span class="term-err">no session selected — open Sessions or type use &lt;agent_id&gt;</span>',
      );
      return;
    }
    if (taskBusy) {
      await appendTerm('<span class="term-err">already waiting on a task — press Ctrl+C to abort</span>');
      return;
    }
    const line = [cmd, ...args].join(" ");
    if (mayProduceLargeOutput(line)) {
      const ok = window.confirm(
        `“${line}” often produces a large amount of output.\n\nRun it anyway?`,
      );
      if (!ok) {
        await appendTerm(`${promptPrefix(selected)} <span class="term-muted">${escapeHtml(line)}</span>`);
        await appendTerm('<span class="term-muted">cancelled — large output not confirmed</span>');
        return;
      }
    }

    taskBusy = true;
    error = "";
    const agent = selected;
    if (agent && !isOnline(agent)) {
      await appendTerm(
        '<span class="term-err">selected session looks offline (no recent check-in). Pick a live session or start the agent.</span>',
      );
    }
    await appendTerm(`${promptPrefix(agent)} <span class="term-muted">${escapeHtml(line)}</span>`);
    const ac = new AbortController();
    activeAbort = ac;
    activeTaskId = "";
    try {
      const result = await submitTaskAndWait(
        selectedAgent,
        cmd,
        args,
        (status) => {
          void appendTerm(`<span class="term-muted">status=${escapeHtml(status)}</span>`);
        },
        (taskId) => {
          activeTaskId = taskId;
          void appendTerm(`<span class="term-path">task ${escapeHtml(taskId)}</span>`);
        },
        ac.signal,
      );
      if (result.stdout) {
        await appendTerm(escapeHtml(result.stdout.replace(/\n$/, "")));
      }
      if (result.stderr) {
        await appendTerm(`<span class="term-err">${escapeHtml(result.stderr.replace(/\n$/, ""))}</span>`);
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

    const parts = trimmed.split(/\s+/);
    const head = parts[0].toLowerCase();
    const rest = parts.slice(1);

    if (head === "clear" || head === "cls") {
      bootConsoleBanner(selected);
      return;
    }
    if (head === "help") {
      await appendTerm(`${promptPrefix(selected)} <span class="term-muted">help</span>`);
      await appendTerm('<span class="term-muted">Agent builtins: whoami, echo, hostname, pwd, cd</span>');
      await appendTerm('<span class="term-muted">Shells: shell|cmd|powershell &lt;line&gt; · or type a native command (dir, ls, …)</span>');
      await appendTerm('<span class="term-muted">Working directory starts at C:\\ or / and persists with cd.</span>');
      await appendTerm('<span class="term-muted">Ctrl+C aborts a running task. Wide listings (dir/tree/…) ask for confirmation first.</span>');
      await appendTerm('<span class="term-muted">Local: help, clear, sessions [query], use &lt;agent_id&gt;</span>');
      return;
    }
    if (head === "sessions") {
      const q = rest.join(" ");
      await appendTerm(`${promptPrefix(selected)} <span class="term-muted">${escapeHtml(trimmed)}</span>`);
      const res = await listSessions({ pageSize: 20, query: q || undefined });
      if (res.sessions.length === 0) {
        await appendTerm('<span class="term-muted">no sessions</span>');
        return;
      }
      await appendTerm(`<span class="term-muted">${res.totalCount} total · showing ${res.sessions.length}</span>`);
      for (const s of res.sessions) {
        const mark = s.agentId === selectedAgent ? "*" : " ";
        await appendTerm(
          `<span class="term-muted">${mark}</span> <span class="term-path">${escapeHtml(s.agentId)}</span>  ${escapeHtml(userAtHost(s.username, s.hostname))}  ${escapeHtml(s.os)}/${escapeHtml(s.arch)}`,
        );
      }
      return;
    }
    if (head === "use") {
      await appendTerm(`${promptPrefix(selected)} <span class="term-muted">${escapeHtml(trimmed)}</span>`);
      const id = rest[0] ?? "";
      const res = await listSessions({ pageSize: 20, query: id });
      const match =
        res.sessions.find((s) => s.agentId === id) ||
        res.sessions.find((s) => s.agentId.startsWith(id)) ||
        res.sessions.find((s) => s.hostname.toLowerCase() === id.toLowerCase());
      if (!match) {
        await appendTerm('<span class="term-err">unknown session — run sessions</span>');
        return;
      }
      selectSession(match);
      await appendTerm(`<span class="term-muted">using ${escapeHtml(match.agentId)}</span>`);
      await appendTerm(`<span class="term-muted">cwd ${escapeHtml(formatPromptPath(match.cwd || guessDefaultCwd(match.os), pathDisplay) || match.cwd || "")}</span>`);
      return;
    }

    await runRemote(parts[0], rest);
    // Pull updated cwd after commands that may change it.
    if (selectedAgent) {
      const res = await listSessions({ pageSize: 5, query: selectedAgent });
      const updated = res.sessions.find((s) => s.agentId === selectedAgent);
      if (updated) selectSession(updated);
    }
  }

  async function onTermKeydown(e: KeyboardEvent) {
    if (e.key === "c" && (e.ctrlKey || e.metaKey)) {
      if (taskBusy && activeAbort) {
        e.preventDefault();
        activeAbort.abort();
        await appendTerm('<span class="term-muted">interrupt sent — waiting for agent to stop…</span>');
      }
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
      termLines = [];
    }
  }

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
    if (nav === "console") {
      if (!consoleBooted || termLines.length === 0) {
        bootConsoleBanner(selected);
      }
      focusTerminal();
      void loadPicker(pickerQuery.trim());
    }
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
      <button class="nav-item" class:active={nav === "settings"} title="Settings" onclick={() => (nav = "settings")}>
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3" />
          <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
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
              <button class="ghost-btn" onclick={() => bootConsoleBanner(selected)}>Clear</button>
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
                <input
                  class="term-input"
                  bind:this={inputEl}
                  bind:value={termInput}
                  disabled={taskBusy}
                  spellcheck="false"
                  autocomplete="off"
                  autocapitalize="off"
                  placeholder={taskBusy ? "waiting for task result…" : "type a command"}
                  onkeydown={onTermKeydown}
                />
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
        {:else}
          <div class="surface">
            <div class="heading">
              <div>
                <h2>Settings</h2>
                <p>Operator console preferences for this browser.</p>
              </div>
            </div>
            <div class="pane-body settings">
              <section class="settings-section">
                <h3>Terminal</h3>
                <p class="muted">Controls how the working directory appears in the prompt.</p>
                <div class="setting-row">
                  <span>Path in prompt</span>
                  <div class="setting-options">
                    <button
                      class="ghost-btn"
                      class:active-setting={pathDisplay === "full"}
                      onclick={() => setPathDisplay("full")}>Full path</button
                    >
                    <button
                      class="ghost-btn"
                      class:active-setting={pathDisplay === "partial"}
                      onclick={() => setPathDisplay("partial")}>Partial path</button
                    >
                  </div>
                </div>
                <div class="command-preview">
                  <div class="label">Preview</div>
                  <code class="mono"
                    >{accountName(selected?.username || "user", selected?.hostname || "host")}@{selected?.hostname ||
                      "host"}:{formatPromptPath(
                      selected?.cwd || guessDefaultCwd(selected?.os),
                      pathDisplay,
                    ) || (selected?.os?.includes("windows") ? "C:\\" : "/")}
                    $</code
                  >
                </div>
              </section>
            </div>
          </div>
        {/if}
      </div>

      <footer class="footer-bar">
        <span><span class="live">●</span> {totalCount.toLocaleString()} session{totalCount === 1 ? "" : "s"}</span>
        <span>↑↓ history · Ctrl+L clear</span>
      </footer>
    </section>
  </div>

  {#if error}
    <p class="error-banner">{error}</p>
  {/if}
</div>
