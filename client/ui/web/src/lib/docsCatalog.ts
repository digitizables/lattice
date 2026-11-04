import developersGettingStarted from "@docs/developers/getting-started.md?raw";
import operatorsElevate from "@docs/operators/elevate.md?raw";
import operatorsPersistence from "@docs/operators/persistence.md?raw";
import operatorsQuickstart from "@docs/operators/quickstart.md?raw";

export type DocSetId = "operators" | "developers";

export type DocEntry = {
  id: string;
  title: string;
  description: string;
  /** Basename used by relative markdown links, e.g. persistence.md */
  file: string;
  markdown: string;
};

export type DocSet = {
  id: DocSetId;
  label: string;
  description: string;
  docs: DocEntry[];
};

export const docSets: DocSet[] = [
  {
    id: "operators",
    label: "Operators",
    description: "Run the console, task agents, persist, elevate, and clean up.",
    docs: [
      {
        id: "operators/quickstart",
        title: "Quickstart",
        description: "Start Postgres, teamserver, lab agent, CLI, and the web UI.",
        file: "quickstart.md",
        markdown: operatorsQuickstart,
      },
      {
        id: "operators/persistence",
        title: "Persistence",
        description: "Keel, Harbor, Splice, and Ash cleanup on target hosts.",
        file: "persistence.md",
        markdown: operatorsPersistence,
      },
      {
        id: "operators/elevate",
        title: "Elevation modules",
        description: "Publish and task bring-your-own privilege modules.",
        file: "elevate.md",
        markdown: operatorsElevate,
      },
    ],
  },
  {
    id: "developers",
    label: "Developers",
    description: "Build order, stack, and how the monorepo fits together.",
    docs: [
      {
        id: "developers/getting-started",
        title: "Getting started",
        description: "Stack, progress, lab run, and env defaults.",
        file: "getting-started.md",
        markdown: developersGettingStarted,
      },
    ],
  },
];

export function findDocSet(id: DocSetId): DocSet {
  return docSets.find((s) => s.id === id) ?? docSets[0];
}

export function findDoc(setId: DocSetId, docId: string): DocEntry {
  const set = findDocSet(setId);
  return set.docs.find((d) => d.id === docId) ?? set.docs[0];
}

/** Resolve a relative markdown href within the current set. */
export function resolveDocLink(setId: DocSetId, href: string): DocEntry | null {
  const clean = href.split("#")[0].replace(/^\.\//, "").trim();
  if (!clean) return null;
  const base = clean.split("/").pop() ?? clean;
  const set = findDocSet(setId);
  return set.docs.find((d) => d.file === base || d.file === clean) ?? null;
}
