export type Session = {
  agentId: string;
  hostname: string;
  username: string;
  os: string;
  arch: string;
  lastSeen?: string;
};

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
};

const servicePrefix = "/lattice.operator.v1.OperatorService";
const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder();

async function connectUnary<TReq extends object, TRes>(
  method: string,
  body: TReq,
): Promise<TRes> {
  const res = await fetch(`${servicePrefix}/${method}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body: JSON.stringify(body),
  });
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

function encodeConnectJSONFrame(message: object): Uint8Array {
  const payload = textEncoder.encode(JSON.stringify(message));
  const out = new Uint8Array(5 + payload.length);
  out[0] = 0;
  const len = payload.length;
  out[1] = (len >>> 24) & 0xff;
  out[2] = (len >>> 16) & 0xff;
  out[3] = (len >>> 8) & 0xff;
  out[4] = len & 0xff;
  out.set(payload, 5);
  return out;
}

type ConnectFrame = { flags: number; json: Record<string, unknown> };

async function* readConnectJSONFrames(
  reader: ReadableStreamDefaultReader<Uint8Array>,
): AsyncGenerator<ConnectFrame> {
  let buffer = new Uint8Array(0);
  while (true) {
    const { done, value } = await reader.read();
    if (value && value.length) {
      const merged = new Uint8Array(buffer.length + value.length);
      merged.set(buffer);
      merged.set(value, buffer.length);
      buffer = merged;
    }
    while (buffer.length >= 5) {
      const flags = buffer[0];
      const len = ((buffer[1] << 24) | (buffer[2] << 16) | (buffer[3] << 8) | buffer[4]) >>> 0;
      if (buffer.length < 5 + len) break;
      const slice = buffer.slice(5, 5 + len);
      buffer = buffer.slice(5 + len);
      const raw = textDecoder.decode(slice);
      let json: Record<string, unknown> = {};
      if (raw) {
        try {
          json = JSON.parse(raw) as Record<string, unknown>;
        } catch {
          json = { raw };
        }
      }
      yield { flags, json };
      if (flags & 0x02) {
        return;
      }
    }
    if (done) return;
  }
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
    lastSeen: lastSeenOut,
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

export async function deleteSession(agentId: string): Promise<void> {
  await connectUnary<{ agentId: string }, Record<string, never>>("DeleteSession", { agentId });
}

/**
 * Opens StreamEvents first (Connect streaming content-type), then submits the task.
 */
export async function submitTaskAndWait(
  agentId: string,
  name: string,
  args: string[],
  onStatus?: (status: string) => void,
  onTaskId?: (taskId: string) => void,
  signal?: AbortSignal,
): Promise<TaskUpdated> {
  const res = await fetch(`${servicePrefix}/StreamEvents`, {
    method: "POST",
    headers: {
      "Content-Type": "application/connect+json",
      "Connect-Protocol-Version": "1",
    },
    body: encodeConnectJSONFrame({ agentId }),
    signal,
  });
  if (!res.ok || !res.body) {
    throw new Error(formatConnectError(await res.text(), res.statusText));
  }

  const taskId = await submitTask(agentId, name, args);
  onTaskId?.(taskId);

  const reader = res.body.getReader();
  for await (const frame of readConnectJSONFrames(reader)) {
    if (frame.flags & 0x02) {
      const errMsg = frame.json.message;
      if (typeof errMsg === "string" && errMsg) {
        throw new Error(errMsg);
      }
      break;
    }
    const taskUpdated = (frame.json.taskUpdated ?? frame.json.task_updated) as
      | Record<string, unknown>
      | undefined;
    if (!taskUpdated) continue;
    const updated: TaskUpdated = {
      taskId: String(taskUpdated.taskId ?? taskUpdated.task_id ?? ""),
      agentId: String(taskUpdated.agentId ?? taskUpdated.agent_id ?? ""),
      status: String(taskUpdated.status ?? ""),
      stdout: String(taskUpdated.stdout ?? ""),
      stderr: String(taskUpdated.stderr ?? ""),
      exitCode: Number(taskUpdated.exitCode ?? taskUpdated.exit_code ?? 0),
    };
    if (updated.taskId !== taskId) continue;
    onStatus?.(updated.status);
    if (updated.status === "completed" || updated.status === "failed") {
      await reader.cancel().catch(() => undefined);
      return updated;
    }
  }
  throw new Error("stream ended before task completed");
}
