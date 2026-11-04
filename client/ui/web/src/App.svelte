<script lang="ts">
  import { tick } from "svelte";
  import { listSessions, submitTaskAndWait, type Session } from "./lib/api";

  type NavId = "sessions" | "console" | "listeners" | "settings";

  let sessions = $state<Session[]>([]);
  let error = $state("");
  let loading = $state(false);
  let nav = $state<NavId>("sessions");
  let selectedAgent = $state("");
  let command = $state("whoami");
  let argsText = $state("");
  let termLines = $state<string[]>([
    '<span class="term-muted">Lattice operator console ready.</span>',
    '<span class="term-muted">Type a command below and press Enter. Try: help</span>',
  ]);
  let dockOpen = $state(true);
  let sidebarExpanded = $state(false);
  let termInput = $state("");
  let history = $state<string[]>([]);
  let historyIdx = $state(-1);
  let termEl: HTMLDivElement | undefined = $state();
  let inputEl: HTMLInputElement | undefined = $state();

  const selected = $derived(sessions.find((s) => s.agentId === selectedAgent));

  const preview = $derived.by(() => {
    const args = argsText.trim().split(/\s+/).filter(Boolean);
    const parts = [command.trim(), ...args].filter(Boolean);
    return parts.join(" ") || "whoami";
  });

  function escapeHtml(s: string) {
    return s
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

  function promptPrefix(agent?: Session) {
    if (!agent) return `<span class="term-prompt">lattice&gt;</span>`;
    return `<span class="term-identity">${escapeHtml(agent.username)}</span>@<span class="term-path">${escapeHtml(agent.hostname)}</span> <span class="term-prompt">$</span>`;
  }

  async function appendTerm(html: string) {
    termLines = [...termLines, html];
    await tick();
    if (termEl) termEl.scrollTop = termEl.scrollHeight;
  }

  function focusTerminal() {
    dockOpen = true;
    void tick().then(() => inputEl?.focus());
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
      await appendTerm('<span class="term-err">no session selected — use &lt;agent_id&gt; or pick one in Sessions</span>');
      return;
    }
    loading = true;
    error = "";
    dockOpen = true;
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

  async function runTaskFromForm() {
    if (!command.trim()) return;
    const args = argsText.trim().split(/\s+/).filter(Boolean);
    await runRemote(command.trim(), args);
  }

  async function handleLocalOrRemote(raw: string) {
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
      await appendTerm('<span class="term-muted">Builtins: whoami, echo, hostname</span>');
      await appendTerm('<span class="term-muted">Local: help, clear, sessions, use &lt;agent_id&gt;, dock</span>');
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
          `<span class="term-muted">${mark}</span> <span class="term-path">${escapeHtml(s.agentId)}</span>  ${escapeHtml(s.username)}@${escapeHtml(s.hostname)}  ${escapeHtml(s.os)}/${escapeHtml(s.arch)}`,
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
    if (head === "dock") {
      dockOpen = !dockOpen;
      if (dockOpen) focusTerminal();
      return;
    }

    command = parts[0];
    argsText = rest.join(" ");
    await runRemote(parts[0], rest);
  }

  async function onTermKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      const value = termInput;
      termInput = "";
      await handleLocalOrRemote(value);
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
    if (dockOpen) focusTerminal();
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
        <button
          class="nav-item"
          class:active={nav === "console"}
          title="Console"
          onclick={() => {
            nav = "console";
            focusTerminal();
          }}
        >
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
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3" /><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" /></svg>
        <span class="nav-label">Settings</span>
      </button>
    </aside>

    <section class="workspace">
      <div class="workspace-tabs">
        <button class="folder-tab" class:active={nav === "sessions"} onclick={() => (nav = "sessions")}>Sessions</button>
        <button
          class="folder-tab"
          class:active={nav === "console"}
          onclick={() => {
            nav = "console";
            focusTerminal();
          }}>Console</button
        >
        <button class="folder-tab" class:active={nav === "listeners"} onclick={() => (nav = "listeners")}>Listeners</button>
        <button class="folder-tab" class:active={nav === "settings"} onclick={() => (nav = "settings")}>Settings</button>
      </div>

      <div class="workspace-body">
        {#if nav === "sessions"}
          <div class="surface">
            <div class="heading">
              <h2>Sessions</h2>
              <p>Active agent callbacks. Double-click a row or type in the terminal dock.</p>
            </div>
            <div class="split">
              <div class="pane">
                <div class="pane-head">
                  <h3>Active agents</h3>
                  <button class="ghost-btn" onclick={() => refresh()} disabled={loading}>Refresh</button>
                </div>
                <div class="pane-body">
                  {#if sessions.length === 0}
                    <div class="empty">
                      <strong>No sessions yet</strong>
                      <span>Start an agent against the HTTPS channel to populate this list.</span>
                    </div>
                  {:else}
                    <table class="sessions">
                      <thead>
                        <tr>
                          <th>Agent</th>
                          <th>User@Host</th>
                          <th>Platform</th>
                          <th>Last seen</th>
                        </tr>
                      </thead>
                      <tbody>
                        {#each sessions as s}
                          <tr
                            class:selected={selectedAgent === s.agentId}
                            onclick={() => {
                              selectedAgent = s.agentId;
                            }}
                            ondblclick={() => {
                              selectedAgent = s.agentId;
                              nav = "console";
                              focusTerminal();
                            }}
                          >
                            <td class="mono">{s.agentId.slice(0, 8)}…</td>
                            <td>{s.username}@{s.hostname}</td>
                            <td>{s.os}/{s.arch}</td>
                            <td class="mono">{s.lastSeen ?? "—"}</td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  {/if}
                </div>
              </div>
              <div class="pane">
                <div class="pane-head">
                  <h3>Quick task</h3>
                </div>
                <div class="pane-body">
                  <div class="field">
                    <label for="agent">Agent</label>
                    <select id="agent" bind:value={selectedAgent}>
                      <option value="">Select session…</option>
                      {#each sessions as s}
                        <option value={s.agentId}>{s.username}@{s.hostname}</option>
                      {/each}
                    </select>
                  </div>
                  <div class="field-row">
                    <div class="field">
                      <label for="cmd">Command</label>
                      <input id="cmd" bind:value={command} placeholder="whoami" />
                    </div>
                    <div class="field">
                      <label for="args">Args</label>
                      <input id="args" bind:value={argsText} placeholder="optional" />
                    </div>
                  </div>
                  <div class="command-preview">
                    <div class="label">Command preview</div>
                    <code class="mono">{preview}</code>
                  </div>
                  <div class="actions">
                    <button class="primary-btn" onclick={() => runTaskFromForm()} disabled={loading || !selectedAgent}>
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z" /></svg>
                      Run task
                    </button>
                    <button
                      class="secondary-btn"
                      onclick={() => {
                        nav = "console";
                        focusTerminal();
                      }}>Focus terminal</button
                    >
                  </div>
                </div>
              </div>
            </div>
          </div>
        {:else if nav === "console"}
          <div class="surface">
            <div class="heading">
              <h2>Console</h2>
              <p>Type directly in the dock below. Up/Down cycles history. Ctrl+L clears.</p>
            </div>
            <div class="pane-body">
              <div class="field-row">
                <div class="field">
                  <label for="agent2">Agent</label>
                  <select id="agent2" bind:value={selectedAgent}>
                    <option value="">Select session…</option>
                    {#each sessions as s}
                      <option value={s.agentId}>{s.username}@{s.hostname} · {s.agentId.slice(0, 8)}</option>
                    {/each}
                  </select>
                </div>
                <div class="field">
                  <label for="cmd2">Quick command</label>
                  <input
                    id="cmd2"
                    bind:value={command}
                    onkeydown={(e) => {
                      if (e.key === "Enter") void runTaskFromForm();
                    }}
                  />
                </div>
              </div>
              <div class="field">
                <label for="args2">Args</label>
                <input id="args2" bind:value={argsText} />
              </div>
              <div class="command-preview">
                <div class="label">Command preview</div>
                <code class="mono">{preview}</code>
              </div>
              <div class="actions">
                <button class="primary-btn" onclick={() => runTaskFromForm()} disabled={loading || !selectedAgent}>Run task</button>
                <button class="ghost-btn" onclick={() => (termLines = [])}>Clear dock</button>
                <button class="ghost-btn" onclick={() => focusTerminal()}>Focus input</button>
              </div>
            </div>
          </div>
        {:else if nav === "listeners"}
          <div class="surface">
            <div class="heading">
              <h2>Listeners</h2>
              <p>Channel endpoints loaded by the extension system.</p>
            </div>
            <div class="empty">
              <strong>HTTPS agent channel</strong>
              <span>Builtin <span class="mono">http</span> channel on <span class="mono">:8443</span> (TLS). Management UI comes next.</span>
            </div>
          </div>
        {:else}
          <div class="surface">
            <div class="heading">
              <h2>Settings</h2>
              <p>Operator API defaults for this browser session.</p>
            </div>
            <div class="pane-body">
              <div class="field">
                <label for="api-base">API base</label>
                <input id="api-base" class="mono" value="http://127.0.0.1:8080 (via Vite proxy)" readonly />
              </div>
              <div class="field">
                <label for="agent-channel">Agent channel</label>
                <input id="agent-channel" class="mono" value="https://127.0.0.1:8443" readonly />
              </div>
            </div>
          </div>
        {/if}

        {#if dockOpen}
          <div class="terminal-dock">
            <div class="dock-tabs">
              <button class="folder-tab active">Terminal</button>
            </div>
            <div class="terminal-surface">
              <div class="session-toolbar">
                <span>{selected ? `${selected.username}@${selected.hostname}` : "no session — type use <id>"}</span>
                <button class="ghost-btn" onclick={() => (dockOpen = false)}>Hide</button>
              </div>
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
          <button class="primary-btn show-dock" onclick={() => focusTerminal()}>Show terminal</button>
        {/if}
      </div>

      <footer class="footer-bar">
        <span
          ><span class="live">●</span> UTF-8 · Connect JSON · StreamEvents · click dock to type</span
        >
        <span>{sessions.length} session{sessions.length === 1 ? "" : "s"} · ↑↓ history · Ctrl+L clear</span>
      </footer>
    </section>
  </div>

  {#if error}
    <p class="error-banner">{error}</p>
  {/if}
</div>
