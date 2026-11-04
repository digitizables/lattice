<script lang="ts">
  import { listSessions, userAtHost, type PathDisplay, type Session } from "./api";
  import TerminalPane from "./TerminalPane.svelte";
  import {
    createPane,
    createTab,
    tabLabel,
    type SplitMode,
    type TermPane,
    type TermTab,
  } from "./terminalModel";

  let {
    pathDisplay,
    openAgentId = $bindable(null as string | null),
  }: {
    pathDisplay: PathDisplay;
    openAgentId?: string | null;
  } = $props();

  const first = createTab("");
  let panes = $state<Record<string, TermPane>>({ [first.pane.id]: first.pane });
  let tabs = $state<TermTab[]>([first.tab]);
  let activeTabId = $state(first.tab.id);
  let pickerSessions = $state<Session[]>([]);
  let pickerQueryByPane = $state<Record<string, string>>({ [first.pane.id]: "" });
  let searchTimer: ReturnType<typeof setTimeout> | undefined;

  const activeTab = $derived(tabs.find((t) => t.id === activeTabId) ?? tabs[0]);
  const activePanes = $derived(
    (activeTab?.paneIds ?? [])
      .map((id) => panes[id])
      .filter((p): p is TermPane => Boolean(p)),
  );

  function sessionLabel(agentId: string): string | undefined {
    if (!agentId) return undefined;
    const s = pickerSessions.find((x) => x.agentId === agentId);
    if (!s) return undefined;
    return userAtHost(s.username, s.hostname);
  }

  function withRenamedTabs(list: TermTab[]): TermTab[] {
    return list.map((tab, idx) => {
      const primary = panes[tab.activePaneId] ?? panes[tab.paneIds[0]];
      return {
        ...tab,
        name: tabLabel(idx, sessionLabel(primary?.agentId ?? "")),
      };
    });
  }

  async function loadPicker(q = "") {
    try {
      const res = await listSessions({ pageSize: 30, query: q || undefined });
      pickerSessions = res.sessions;
      tabs = withRenamedTabs(tabs);
    } catch {
      // keep prior
    }
  }

  function schedulePickerSearch(paneId: string) {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      void loadPicker(pickerQueryByPane[paneId]?.trim() ?? "");
    }, 250);
  }

  function setActiveTab(id: string) {
    activeTabId = id;
  }

  function addTab(agentId = "") {
    const { tab, pane } = createTab(agentId);
    panes = { ...panes, [pane.id]: pane };
    pickerQueryByPane = { ...pickerQueryByPane, [pane.id]: "" };
    tabs = withRenamedTabs([...tabs, tab]);
    activeTabId = tab.id;
    void loadPicker("");
  }

  function closeTab(tabId: string) {
    const tab = tabs.find((t) => t.id === tabId);
    if (!tab) return;
    const nextPanes = { ...panes };
    const nextQueries = { ...pickerQueryByPane };
    for (const pid of tab.paneIds) {
      delete nextPanes[pid];
      delete nextQueries[pid];
    }
    panes = nextPanes;
    pickerQueryByPane = nextQueries;

    const remaining = tabs.filter((t) => t.id !== tabId);
    if (remaining.length === 0) {
      const fresh = createTab("");
      panes = { [fresh.pane.id]: fresh.pane };
      pickerQueryByPane = { [fresh.pane.id]: "" };
      tabs = withRenamedTabs([fresh.tab]);
      activeTabId = fresh.tab.id;
      return;
    }
    tabs = withRenamedTabs(remaining);
    if (activeTabId === tabId) {
      activeTabId = remaining[Math.max(0, remaining.length - 1)].id;
    }
  }

  function splitActive(mode: Exclude<SplitMode, "none">) {
    const tab = tabs.find((t) => t.id === activeTabId);
    if (!tab) return;
    if (tab.paneIds.length >= 2) {
      tabs = withRenamedTabs(
        tabs.map((t) => (t.id === tab.id ? { ...t, split: mode } : t)),
      );
      return;
    }
    const source = panes[tab.activePaneId] ?? panes[tab.paneIds[0]];
    const pane = createPane(source?.agentId ?? "");
    panes = { ...panes, [pane.id]: pane };
    pickerQueryByPane = { ...pickerQueryByPane, [pane.id]: "" };
    tabs = withRenamedTabs(
      tabs.map((t) =>
        t.id === tab.id
          ? {
              ...t,
              split: mode,
              paneIds: [...t.paneIds, pane.id],
              activePaneId: pane.id,
            }
          : t,
      ),
    );
    void loadPicker("");
  }

  function closePane(tabId: string, paneId: string) {
    const tab = tabs.find((t) => t.id === tabId);
    if (!tab) return;
    if (tab.paneIds.length <= 1) {
      closeTab(tabId);
      return;
    }
    const nextIds = tab.paneIds.filter((id) => id !== paneId);
    const nextPanes = { ...panes };
    delete nextPanes[paneId];
    panes = nextPanes;
    const q = { ...pickerQueryByPane };
    delete q[paneId];
    pickerQueryByPane = q;
    tabs = withRenamedTabs(
      tabs.map((t) =>
        t.id === tabId
          ? {
              ...t,
              split: "none" as SplitMode,
              paneIds: nextIds,
              activePaneId: nextIds.includes(t.activePaneId) ? t.activePaneId : nextIds[0],
            }
          : t,
      ),
    );
  }

  function focusPane(tabId: string, paneId: string) {
    tabs = tabs.map((t) => (t.id === tabId ? { ...t, activePaneId: paneId } : t));
    activeTabId = tabId;
  }

  function setPaneAgent(paneId: string, agentId: string) {
    const pane = panes[paneId];
    if (!pane) return;
    panes = { ...panes, [paneId]: { ...pane, agentId } };
    tabs = withRenamedTabs(tabs);
  }

  function onSessionUpdated(s: Session) {
    const idx = pickerSessions.findIndex((x) => x.agentId === s.agentId);
    if (idx >= 0) {
      pickerSessions = [
        ...pickerSessions.slice(0, idx),
        s,
        ...pickerSessions.slice(idx + 1),
      ];
    } else {
      pickerSessions = [s, ...pickerSessions];
    }
    tabs = withRenamedTabs(tabs);
  }

  function paneQuery(paneId: string): string {
    return pickerQueryByPane[paneId] ?? "";
  }

  function setPaneQuery(paneId: string, value: string) {
    pickerQueryByPane = { ...pickerQueryByPane, [paneId]: value };
  }

  $effect(() => {
    void loadPicker("");
  });

  $effect(() => {
    if (!openAgentId) return;
    const agentId = openAgentId;
    openAgentId = null;
    const tab = tabs.find((t) => t.id === activeTabId) ?? tabs[0];
    const paneId = tab?.activePaneId ?? tab?.paneIds[0];
    if (paneId) setPaneAgent(paneId, agentId);
    else addTab(agentId);
    void loadPicker("");
  });
</script>

<div class="surface console-surface">
  <div class="term-tabbar">
    <div class="term-tabs" role="tablist" aria-label="Terminal tabs">
      {#each tabs as tab, i (tab.id)}
        <div class="term-tab" class:active={tab.id === activeTabId} role="tab" aria-selected={tab.id === activeTabId}>
          <button type="button" class="term-tab-main" onclick={() => setActiveTab(tab.id)}>
            <span class="term-tab-label">{tab.name || tabLabel(i)}</span>
          </button>
          <button
            type="button"
            class="term-tab-close"
            title="Close tab"
            aria-label="Close tab"
            onclick={() => closeTab(tab.id)}
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              ><path d="M18 6 6 18M6 6l12 12" /></svg
            >
          </button>
        </div>
      {/each}
      <button
        type="button"
        class="ghost-btn icon-btn term-tab-add"
        title="New terminal tab"
        aria-label="New terminal tab"
        onclick={() => addTab(panes[activeTab?.activePaneId ?? ""]?.agentId ?? "")}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
          ><path d="M5 12h14M12 5v14" /></svg
        >
      </button>
    </div>
    <div class="term-tab-actions">
      <button
        type="button"
        class="ghost-btn icon-btn"
        class:active={activeTab?.split === "vertical"}
        title="Split right"
        aria-label="Split right"
        onclick={() => splitActive("vertical")}
      >
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
          ><rect width="18" height="18" x="3" y="3" rx="2" /><path d="M12 3v18" /></svg
        >
      </button>
      <button
        type="button"
        class="ghost-btn icon-btn"
        class:active={activeTab?.split === "horizontal"}
        title="Split down"
        aria-label="Split down"
        onclick={() => splitActive("horizontal")}
      >
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
          ><rect width="18" height="18" x="3" y="3" rx="2" /><path d="M3 12h18" /></svg
        >
      </button>
    </div>
  </div>

  {#if activeTab}
    <div
      class="term-split"
      class:split-vertical={activeTab.split === "vertical" && activePanes.length > 1}
      class:split-horizontal={activeTab.split === "horizontal" && activePanes.length > 1}
    >
      {#each activePanes as pane, idx (pane.id)}
        <TerminalPane
          agentId={pane.agentId}
          pathDisplay={pathDisplay}
          pickerSessions={pickerSessions}
          pickerQuery={paneQuery(pane.id)}
          active={activeTab.activePaneId === pane.id}
          showClose={activePanes.length > 1}
          title={activePanes.length > 1 ? `Pane ${idx + 1}` : activeTab.name}
          onFocus={() => focusPane(activeTab.id, pane.id)}
          onClose={() => closePane(activeTab.id, pane.id)}
          onPickerQuery={(q) => setPaneQuery(pane.id, q)}
          onPickerSearch={() => schedulePickerSearch(pane.id)}
          onAgentChange={(id) => setPaneAgent(pane.id, id)}
          onSessionUpdated={onSessionUpdated}
        />
      {/each}
    </div>
  {/if}
</div>
