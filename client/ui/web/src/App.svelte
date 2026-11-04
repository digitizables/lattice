<script lang="ts">
  import { tick } from "svelte";
  import { accountName, listSessions, submitTaskAndWait, userAtHost, type Session } from "./lib/api";

  type NavId = "sessions" | "console" | "listeners";

  let sessions = $state<Session[]>([]);
  let error = $state("");
  let loading = $state(false);
  let nav = $state<NavId>("sessions");
  let selectedAgent = $state("");
  let termLines = $state<string[]>([
    '<span class="term-muted">Lattice operator console ready.</span>',
    '<span class="term-muted">Select a session, then type a command. Try: help</span>',
  ]);
  let sidebarExpanded = $state(false);
  let termInput = $state("");
  let history = $state<string[]>([]);
  let historyIdx = $state(-1);
  let termEl: HTMLDivElement | undefined = $state();
  let inputEl: HTMLInputElement | undefined = $state();

  const selected = $derived(sessions.find((s) => s.agentId === selectedAgent));

  function escapeHtml(s: string) {
    return s
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

  function promptPrefix(agent?: Session) {
    if (!agent) return `<span class="term-prompt">lattice&gt;</span>`;
    const user = accountName(agent.username, agent.hostname);
    return `<span class="term-identity">${escapeHtml(user)}</span>@<span class="term-path">${escapeHtml(agent.hostname)}</span> <span class="term-prompt">$</span>`;
  }

  async function appendTerm(html: string) {
    termLines = [...termLines, html];
    await tick();
    if (termEl) termEl.scrollTop = termEl.scrollHeight;
  }

  function focusTerminal() {
    void tick().then(() => inputEl?.focus());
  }

  function openConsole(agentId?: string) {
    if (agentId) selectedAgent = agentId;
    nav = "console";
    focusTerminal();
  }

  async function refresh() {
    loading = true;
    error = "";
    try {
      sessions = await listSessions();
      if (!selectedAgent && sessions.length > 0) {
        selectedAgent = sessions[0].agentId;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

  async function runRemote(cmd: string, args: string[]) {
    if (!selectedAgent) {
      await appendTerm(
        '<span class="term-err">no session selected — open Sessions or type use &lt;agent_id&gt;</span>',
      );
      return;
    }
    loading = true;
    error = "";
    const agent = selected;
    const line = [cmd, ...args].join(" ");
    await appendTerm(`${promptPrefix(agent)} <span class="term-muted">${escapeHtml(line)}</span>`);
    try {
      const result = await submitTaskAndWait(
        selectedAgent,
        cmd,
        args,
        (status) => {
          void appendTerm(`<span class="term-muted">status=${escapeHtml(status)}</span>`);
        },
        (taskId) => {
          void appendTerm(`<span class="term-path">task ${escapeHtml(taskId)}</span>`);
        },
      );
      if (result.stdout) {
        await appendTerm(escapeHtml(result.stdout.replace(/\n$/, "")));
      }
      if (result.stderr) {
        await appendTerm(`<span class="term-err">${escapeHtml(result.stderr.replace(/\n$/, ""))}</span>`);
      }
      await refresh();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      error = msg;
      await appendTerm(`<span class="term-err">${escapeHtml(msg)}</span>`);
    } finally {
      loading = false;
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
      termLines = [];
      return;
    }
    if (head === "help") {
      await appendTerm(`${promptPrefix(selected)} <span class="term-muted">help</span>`);
      await appendTerm('<span class="term-muted">Agent: whoami, echo, hostname</span>');
      await appendTerm('<span class="term-muted">Local: help, clear, sessions, use &lt;agent_id&gt;</span>');
      return;
    }
    if (head === "sessions") {
      await appendTerm(`${promptPrefix(selected)} <span class="term-muted">sessions</span>`);
      await refresh();
      if (sessions.length === 0) {
        await appendTerm('<span class="term-muted">no sessions</span>');
        return;
      }
      for (const s of sessions) {
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
      const match =
        sessions.find((s) => s.agentId === id) ||
        sessions.find((s) => s.agentId.startsWith(id)) ||
        sessions.find((s) => s.hostname.toLowerCase() === id.toLowerCase());
      if (!match) {
        await appendTerm('<span class="term-err">unknown session — run sessions</span>');
        return;
      }
      selectedAgent = match.agentId;
      await appendTerm(`<span class="term-muted">using ${escapeHtml(match.agentId)}</span>`);
      return;
    }

    await runRemote(parts[0], rest);
  }

  async function onTermKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
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
    void refresh();
    const id = setInterval(() => {
      void refresh();
    }, 5000);
    return () => clearInterval(id);
  });

  $effect(() => {
    if (nav === "console") focusTerminal();
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
    </aside>

    <section class="workspace">
      <div class="workspace-body">
        {#if nav === "sessions"}
          <div class="surface">
            <div class="heading">
              <div>
                <h2>Sessions</h2>
                <p>Active agent callbacks. Open one to task it in the console.</p>
              </div>
              <button class="ghost-btn" onclick={() => refresh()} disabled={loading}>Refresh</button>
            </div>
            <div class="pane-body grow">
              {#if sessions.length === 0}
                <div class="empty">
                  <strong>No sessions yet</strong>
                  <span>Start an agent against <span class="mono">https://127.0.0.1:8443</span>.</span>
                </div>
              {:else}
                <table class="sessions">
                  <thead>
                    <tr>
                      <th>Agent</th>
                      <th>User@Host</th>
                      <th>Platform</th>
                      <th>Last seen</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each sessions as s}
                      <tr
                        class:selected={selectedAgent === s.agentId}
                        onclick={() => (selectedAgent = s.agentId)}
                        ondblclick={() => openConsole(s.agentId)}
                      >
                        <td class="mono" title={s.agentId}>{s.agentId.slice(0, 8)}…</td>
                        <td>{userAtHost(s.username, s.hostname)}</td>
                        <td>{s.os}/{s.arch}</td>
                        <td class="mono">{s.lastSeen ?? "—"}</td>
                        <td>
                          <button
                            class="ghost-btn"
                            onclick={(e) => {
                              e.stopPropagation();
                              openConsole(s.agentId);
                            }}>Console</button
                          >
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            </div>
          </div>
        {:else if nav === "console"}
          <div class="surface console-surface">
            <div class="console-toolbar">
              <label class="agent-picker" for="agent-select">
                <span>Session</span>
                <select id="agent-select" bind:value={selectedAgent} onchange={() => focusTerminal()}>
                  <option value="">Select session…</option>
                  {#each sessions as s}
                    <option value={s.agentId}>{userAtHost(s.username, s.hostname)}</option>
                  {/each}
                </select>
              </label>
              <button class="ghost-btn" onclick={() => (termLines = [])}>Clear</button>
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
                  disabled={loading}
                  spellcheck="false"
                  autocomplete="off"
                  autocapitalize="off"
                  placeholder={loading ? "waiting for agent…" : "type a command"}
                  onkeydown={onTermKeydown}
                />
              </div>
            </div>
          </div>
        {:else}
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
        {/if}
      </div>

      <footer class="footer-bar">
        <span><span class="live">●</span> {sessions.length} session{sessions.length === 1 ? "" : "s"}</span>
        <span>↑↓ history · Ctrl+L clear</span>
      </footer>
    </section>
  </div>

  {#if error}
    <p class="error-banner">{error}</p>
  {/if}
</div>
