<script lang="ts">
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
    '<span class="term-muted">Select a session, then run a task.</span>',
  ]);
  let dockOpen = $state(true);

  const selected = $derived(sessions.find((s) => s.agentId === selectedAgent));

  const preview = $derived.by(() => {
    const args = argsText.trim().split(/\s+/).filter(Boolean);
    const parts = [command.trim(), ...args].filter(Boolean);
    return parts.join(" ") || "whoami";
  });

  function appendTerm(html: string) {
    termLines = [...termLines, html];
  }

  function promptPrefix(agent?: Session) {
    if (!agent) return `<span class="term-prompt">lattice&gt;</span>`;
    return `<span class="term-identity">${escapeHtml(agent.username)}</span>@<span class="term-path">${escapeHtml(agent.hostname)}</span> <span class="term-prompt">$</span>`;
  }

  function escapeHtml(s: string) {
    return s
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
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

  async function runTask() {
    if (!selectedAgent || !command.trim()) return;
    loading = true;
    error = "";
    dockOpen = true;
    const args = argsText.trim().split(/\s+/).filter(Boolean);
    const agent = selected;
    appendTerm(`${promptPrefix(agent)} <span class="term-muted">${escapeHtml(preview)}</span>`);
    try {
      const result = await submitTaskAndWait(
        selectedAgent,
        command.trim(),
        args,
        (status) => {
          appendTerm(`<span class="term-muted">status=${escapeHtml(status)}</span>`);
        },
        (taskId) => {
          appendTerm(`<span class="term-path">task ${escapeHtml(taskId)}</span>`);
        },
      );
      if (result.stdout) {
        appendTerm(escapeHtml(result.stdout.replace(/\n$/, "")));
      }
      if (result.stderr) {
        appendTerm(`<span class="term-err">${escapeHtml(result.stderr.replace(/\n$/, ""))}</span>`);
      }
      await refresh();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      error = msg;
      appendTerm(`<span class="term-err">${escapeHtml(msg)}</span>`);
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    void refresh();
    const id = setInterval(() => {
      void refresh();
    }, 5000);
    return () => clearInterval(id);
  });
</script>

<div class="app-window">
  <header class="titlebar">
    <div class="title-block">
      <div class="brand">Lattice</div>
      <div class="descriptor">Operator console</div>
    </div>
    <div class="window-controls" aria-hidden="true">
      <button class="win-btn" tabindex="-1" title="Minimize">
        <svg width="10" height="10" viewBox="0 0 10 10"><rect y="4.5" width="10" height="1" fill="currentColor" /></svg>
      </button>
      <button class="win-btn" tabindex="-1" title="Maximize">
        <svg width="10" height="10" viewBox="0 0 10 10"><rect x="0.5" y="0.5" width="9" height="9" fill="none" stroke="currentColor" /></svg>
      </button>
      <button class="win-btn close" tabindex="-1" title="Close">
        <svg width="10" height="10" viewBox="0 0 10 10"><path d="M1 1l8 8M9 1L1 9" stroke="currentColor" stroke-width="1.2" /></svg>
      </button>
    </div>
  </header>

  <div class="shell">
    <aside class="sidebar">
      <div class="sidebar-top">
        <button class="nav-item" title="Collapse" aria-label="Collapse navigation">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 18l-6-6 6-6" /></svg>
        </button>
      </div>
      <nav class="nav-group">
        <button class="nav-item" class:active={nav === "sessions"} title="Sessions" onclick={() => (nav = "sessions")}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8M12 17v4" /></svg>
        </button>
        <button class="nav-item" class:active={nav === "console"} title="Console" onclick={() => (nav = "console")}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg>
        </button>
        <button class="nav-item" class:active={nav === "listeners"} title="Listeners" onclick={() => (nav = "listeners")}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12.55a11 11 0 0 1 14.08 0" /><path d="M1.42 9a16 16 0 0 1 21.16 0" /><path d="M8.53 16.11a6 6 0 0 1 6.95 0" /><circle cx="12" cy="20" r="1" /></svg>
        </button>
      </nav>
      <div class="sidebar-spacer"></div>
      <button class="nav-item" class:active={nav === "settings"} title="Settings" onclick={() => (nav = "settings")}>
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3" /><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" /></svg>
      </button>
    </aside>

    <section class="workspace">
      <div class="workspace-tabs">
        <button class="folder-tab" class:active={nav === "sessions"} onclick={() => (nav = "sessions")}>Sessions</button>
        <button class="folder-tab" class:active={nav === "console"} onclick={() => (nav = "console")}>Console</button>
        {#if selected}
          <button class="folder-tab active" title={selected.agentId}>
            {selected.hostname}
            <span class="tab-x">×</span>
          </button>
        {/if}
        <button class="add-tab" title="New view" onclick={() => refresh()}>+</button>
      </div>

      <div class="workspace-body">
        {#if nav === "sessions"}
          <div class="surface">
            <div class="heading">
              <h2>Sessions</h2>
              <p>Active agent callbacks. Select one to task from the console dock.</p>
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
                              nav = "console";
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
                    <button class="primary-btn" onclick={() => runTask()} disabled={loading || !selectedAgent}>
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z" /></svg>
                      Run task
                    </button>
                    <button class="secondary-btn" onclick={() => (nav = "console")}>Open console</button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        {:else if nav === "console"}
          <div class="surface">
            <div class="heading">
              <h2>Console</h2>
              <p>Issue agent builtins and watch StreamEvents land in the dock.</p>
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
                  <label for="cmd2">Command</label>
                  <input id="cmd2" bind:value={command} />
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
                <button class="primary-btn" onclick={() => runTask()} disabled={loading || !selectedAgent}>Run task</button>
                <button class="ghost-btn" onclick={() => (termLines = [])}>Clear dock</button>
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
                <span>{selected ? `${selected.username}@${selected.hostname}` : "no session"}</span>
                <button class="ghost-btn" onclick={() => (dockOpen = false)}>Hide</button>
              </div>
              <div class="terminal-canvas mono">
                {#each termLines as line}
                  <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                  <div class="term-line">{@html line}</div>
                {/each}
                <div class="term-line">{@html promptPrefix(selected)} <span class="term-muted">█</span></div>
              </div>
            </div>
          </div>
        {/if}
      </div>

      <footer class="footer-bar">
        <span>UTF-8 · Connect JSON · StreamEvents</span>
        <span>{sessions.length} session{sessions.length === 1 ? "" : "s"} · Ctrl+T new tab</span>
      </footer>
    </section>
  </div>

  {#if error}
    <p class="error-banner">{error}</p>
  {/if}
</div>
