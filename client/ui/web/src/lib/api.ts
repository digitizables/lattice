export type Session = {
  agentId: string;
  hostname: string;
  username: string;
  os: string;
  arch: string;
  lastSeen?: string;
};

export type TaskUpdated = {
  taskId: string;
  agentId: string;
  status: string;
  stdout: string;
  stderr: string;
  exitCode: number;
};

const servicePrefix = "/lattice.operator.v1.OperatorService";

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
    throw new Error(text || res.statusText);
  }
  return text ? (JSON.parse(text) as TRes) : ({} as TRes);
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

export async function listSessions(): Promise<Session[]> {
  const data = await connectUnary<Record<string, never>, { sessions?: Record<string, unknown>[] }>(
    "ListSessions",
    {},
  );
  return (data.sessions ?? []).map(mapSession);
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

/**
 * Opens StreamEvents first, then runs submit() so the server subscription exists
 * before the task is queued.
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
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body: JSON.stringify({ agentId }),
    signal,
  });
  if (!res.ok || !res.body) {
    throw new Error((await res.text()) || res.statusText);
  }

  const taskId = await submitTask(agentId, name, args);
  onTaskId?.(taskId);

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      throw new Error("stream ended before task completed");
    }
    buffer += decoder.decode(value, { stream: true });

    let idx: number;
    while ((idx = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, idx).trim();
      buffer = buffer.slice(idx + 1);
      if (!line) continue;
      let msg: Record<string, unknown>;
      try {
        msg = JSON.parse(line) as Record<string, unknown>;
      } catch {
        continue;
      }
      const result = msg.result as Record<string, unknown> | undefined;
      const payload = (result ?? msg) as Record<string, unknown>;
      const taskUpdated = (payload.taskUpdated ?? payload.task_updated) as
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
        await reader.cancel();
        return updated;
      }
    }
  }
}
