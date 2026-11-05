export type Session = {
  agentId: string;
  hostname: string;
  username: string;
  os: string;
  arch: string;
  cwd?: string;
  lastSeen?: string;
};

export type PathDisplay = "full" | "partial";

const PATH_DISPLAY_KEY = "lattice.terminal.pathDisplay";

export function loadPathDisplay(): PathDisplay {
  const v = localStorage.getItem(PATH_DISPLAY_KEY);
  return v === "partial" ? "partial" : "full";
}

export function savePathDisplay(mode: PathDisplay) {
  localStorage.setItem(PATH_DISPLAY_KEY, mode);
}

/** Format host cwd for the prompt. */
export function formatPromptPath(cwd: string | undefined, mode: PathDisplay): string {
  if (!cwd) return "";
  const normalized = cwd.replace(/\\/g, "/");
  if (mode === "full") return cwd;
  // Windows drive root or Unix root
  if (/^[a-zA-Z]:[\\/]?$/.test(cwd) || cwd === "/") return cwd;
  const parts = normalized.split("/").filter(Boolean);
  if (parts.length === 0) return cwd;
  const last = parts[parts.length - 1];
  // Preserve Windows drive letter when present
  if (/^[a-zA-Z]:/.test(cwd)) {
    return last;
  }
  return last;
}

/** Windows check-ins often send HOST\\user; avoid HOST\\user@HOST in the UI. */
export function accountName(username: string, hostname: string): string {
  const sep = username.lastIndexOf("\\");
  if (sep < 0) return username;
  const domain = username.slice(0, sep);
  const account = username.slice(sep + 1);
  if (hostname && domain.toLowerCase() === hostname.toLowerCase()) {
    return account || username;
  }
  return username;
}

export function userAtHost(username: string, hostname: string): string {
  const user = accountName(username, hostname);
  if (!hostname) return user;
  if (!user) return hostname;
  return `${user}@${hostname}`;
}

export type TaskUpdated = {
  taskId: string;
  agentId: string;
  status: string;
  stdout: string;
  stderr: string;
  exitCode: number;
};

export type ListSessionsParams = {
  pageSize?: number;
  pageToken?: string;
  query?: string;
};

export type ListSessionsResult = {
  sessions: Session[];
  nextPageToken: string;
  totalCount: number;
  activeCount: number;
};

// Talk to the teamserver directly in lab/dev. Vite's proxy breaks Connect streams
// and can surface as browser "Failed to fetch" while waiting on tasks.
const servicePrefix =
  (import.meta.env.VITE_API_BASE?.replace(/\/$/, "") || "http://127.0.0.1:8080") +
  "/lattice.operator.v1.OperatorService";

async function connectUnary<TReq extends object, TRes>(
  method: string,
  body: TReq,
): Promise<TRes> {
  let res: Response;
  try {
    res = await fetch(`${servicePrefix}/${method}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Connect-Protocol-Version": "1",
      },
      body: JSON.stringify(body),
    });
  } catch {
    throw new Error(
      `Failed to reach teamserver at ${servicePrefix}. Is it running on :8080?`,
    );
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(formatConnectError(text, res.statusText));
  }
  return text ? (JSON.parse(text) as TRes) : ({} as TRes);
}

function formatConnectError(body: string, fallback: string): string {
  if (!body) return fallback;
  try {
    const parsed = JSON.parse(body) as { message?: string; code?: string };
    if (parsed.message) return parsed.message;
  } catch {
    // not JSON
  }
  return body;
}

function mapSession(raw: Record<string, unknown>): Session {
  const lastSeen = raw.lastSeen as { seconds?: string | number } | string | undefined;
  let lastSeenOut: string | undefined;
  if (typeof lastSeen === "string") {
    lastSeenOut = lastSeen;
  } else if (lastSeen && typeof lastSeen === "object" && lastSeen.seconds != null) {
    lastSeenOut = new Date(Number(lastSeen.seconds) * 1000).toISOString();
  }
  return {
    agentId: String(raw.agentId ?? ""),
    hostname: String(raw.hostname ?? ""),
    username: String(raw.username ?? ""),
    os: String(raw.os ?? ""),
    arch: String(raw.arch ?? ""),
    cwd: String(raw.cwd ?? ""),
    lastSeen: lastSeenOut,
  };
}

function mapTask(raw: Record<string, unknown>): TaskUpdated {
  return {
    taskId: String(raw.taskId ?? raw.task_id ?? ""),
    agentId: String(raw.agentId ?? raw.agent_id ?? ""),
    status: String(raw.status ?? ""),
    stdout: String(raw.stdout ?? ""),
    stderr: String(raw.stderr ?? ""),
    exitCode: Number(raw.exitCode ?? raw.exit_code ?? 0),
  };
}

export async function listSessions(params: ListSessionsParams = {}): Promise<ListSessionsResult> {
  const body: Record<string, unknown> = {
    pageSize: params.pageSize ?? 50,
  };
  if (params.pageToken) body.pageToken = params.pageToken;
  if (params.query) body.query = params.query;

  const data = await connectUnary<Record<string, unknown>, Record<string, unknown>>(
    "ListSessions",
    body,
  );
  return {
    sessions: ((data.sessions as Record<string, unknown>[] | undefined) ?? []).map(mapSession),
    nextPageToken: String(data.nextPageToken ?? ""),
    totalCount: Number(data.totalCount ?? 0),
    activeCount: Number(data.activeCount ?? 0),
  };
}

export async function submitTask(
  agentId: string,
  name: string,
  args: string[],
): Promise<string> {
  const data = await connectUnary<
    { agentId: string; name: string; args: string[] },
    { taskId?: string }
  >("SubmitTask", { agentId, name, args });
  return String(data.taskId ?? "");
}

export async function getTask(taskId: string): Promise<TaskUpdated> {
  const data = await connectUnary<{ taskId: string }, { task?: Record<string, unknown> }>(
    "GetTask",
    { taskId },
  );
  return mapTask(data.task ?? {});
}

export async function deleteSession(agentId: string): Promise<void> {
  await connectUnary<{ agentId: string }, Record<string, never>>("DeleteSession", { agentId });
}

export async function cancelTask(taskId: string): Promise<boolean> {
  const data = await connectUnary<{ taskId: string }, { cancelled?: boolean }>("CancelTask", {
    taskId,
  });
  return Boolean(data.cancelled);
}

export type Listener = {
  name: string;
  kind: string;
  bind: string;
  tls: boolean;
  status: string;
  version: string;
};

export async function listListeners(): Promise<Listener[]> {
  const data = await connectUnary<Record<string, never>, { listeners?: Record<string, unknown>[] }>(
    "ListListeners",
    {},
  );
  return ((data.listeners as Record<string, unknown>[] | undefined) ?? []).map((raw) => ({
    name: String(raw.name ?? ""),
    kind: String(raw.kind ?? ""),
    bind: String(raw.bind ?? ""),
    tls: Boolean(raw.tls),
    status: String(raw.status ?? ""),
    version: String(raw.version ?? ""),
  }));
}

/**
 * Submit a task, then poll GetTask until the agent finishes.
 * Pass AbortSignal (Ctrl+C) to cancel remotely and stop waiting.
 * onOutput receives the full stdout snapshot as the agent streams live output
 * (ping, tail, …) while status is still running.
 */
export async function submitTaskAndWait(
  agentId: string,
  name: string,
  args: string[],
  onStatus?: (status: string) => void,
  onTaskId?: (taskId: string) => void,
  signal?: AbortSignal,
  onOutput?: (stdout: string, stderr: string) => void,
): Promise<TaskUpdated> {
  const taskId = await submitTask(agentId, name, args);
  onTaskId?.(taskId);
  onStatus?.("queued");

  const abortRemote = async () => {
    try {
      await cancelTask(taskId);
    } catch {
      // best-effort
    }
  };
  if (signal?.aborted) {
    await abortRemote();
    throw new Error("^C");
  }
  signal?.addEventListener(
    "abort",
    () => {
      void abortRemote();
    },
    { once: true },
  );

  const started = Date.now();
  const maxWaitMs = 2 * 60 * 60 * 1000; // match agent shell backstop
  let lastStatus = "queued";
  let queuedHintAt = 0;
  let lastStdout = "";
  let lastStderr = "";
  let runningSince = 0;

  while (Date.now() - started < maxWaitMs) {
    if (signal?.aborted) {
      await abortRemote();
      // Best-effort final snapshot so streamed lines are not lost.
      try {
        const t = await getTask(taskId);
        if (t.stdout || t.stderr) onOutput?.(t.stdout || "", t.stderr || "");
      } catch {
        // ignore
      }
      throw new Error("^C");
    }
    try {
      await sleep(400, signal);
    } catch {
      await abortRemote();
      throw new Error("^C");
    }
    const task = await getTask(taskId);
    if (signal?.aborted || task.status === "cancelled") {
      if (task.stdout || task.stderr) onOutput?.(task.stdout || "", task.stderr || "");
      throw new Error("^C");
    }
    if (task.status && task.status !== lastStatus) {
      lastStatus = task.status;
      onStatus?.(task.status);
      if (task.status === "running" || task.status === "sent") {
        runningSince = Date.now();
      }
    }
    if (
      (task.status === "running" || task.status === "sent" || task.status === "completed" || task.status === "failed") &&
      (task.stdout !== lastStdout || task.stderr !== lastStderr)
    ) {
      lastStdout = task.stdout || "";
      lastStderr = task.stderr || "";
      onOutput?.(lastStdout, lastStderr);
    }
    if (task.status === "queued" || task.status === "") {
      const waited = Date.now() - started;
      if (waited >= 8000 && waited - queuedHintAt >= 8000) {
        queuedHintAt = waited;
        onStatus?.(
          "queued — no agent has picked this up; is an agent running for this session?",
        );
      }
      if (waited >= 20_000) {
        await abortRemote();
        throw new Error(
          "task still queued after 20s — that session is probably offline. Pick an online session or start the Docker lab agent (packager/docker/docker-compose.lab.yml).",
        );
      }
    }
    if (task.status === "completed" || task.status === "failed") {
      return task;
    }
    // Safety: if "running" forever with no cancel, still respect maxWaitMs via loop.
    void runningSince;
  }
  throw new Error("timed out waiting for agent");
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new Error("^C"));
      return;
    }
    const t = setTimeout(resolve, ms);
    signal?.addEventListener(
      "abort",
      () => {
        clearTimeout(t);
        reject(new Error("^C"));
      },
      { once: true },
    );
  });
}
