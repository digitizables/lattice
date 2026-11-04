export type SplitMode = "none" | "horizontal" | "vertical";

export type TermPane = {
  id: string;
  agentId: string;
};

export type TermTab = {
  id: string;
  name: string;
  split: SplitMode;
  paneIds: string[];
  activePaneId: string;
};

let seq = 0;

export function nextId(prefix: string): string {
  seq += 1;
  return `${prefix}-${seq}-${Date.now().toString(36)}`;
}

export function createPane(agentId = ""): TermPane {
  return { id: nextId("pane"), agentId };
}

export function createTab(agentId = "", name?: string): { tab: TermTab; pane: TermPane } {
  const pane = createPane(agentId);
  const tab: TermTab = {
    id: nextId("tab"),
    name: name ?? "Terminal",
    split: "none",
    paneIds: [pane.id],
    activePaneId: pane.id,
  };
  return { tab, pane };
}

export function tabLabel(index: number, agentLabel?: string): string {
  if (agentLabel) return agentLabel;
  return `Terminal ${index + 1}`;
}
