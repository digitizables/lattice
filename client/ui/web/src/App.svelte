<script lang="ts">
  import { listSessions, submitTaskAndWait, type Session } from "./lib/api";

  let sessions = $state<Session[]>([]);
  let error = $state("");
  let loading = $state(false);
  let selectedAgent = $state("");
  let command = $state("whoami");
  let argsText = $state("");
  let taskLog = $state("");

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
    taskLog = "";
    const args = argsText
      .trim()
      .split(/\s+/)
      .filter(Boolean);
    try {
      const result = await submitTaskAndWait(
        selectedAgent,
        command.trim(),
        args,
        (status) => {
          taskLog += `status=${status}\n`;
        },
        (taskId) => {
          taskLog += `task ${taskId}\n`;
        },
      );
      if (result.stdout) taskLog += result.stdout.endsWith("\n") ? result.stdout : result.stdout + "\n";
      if (result.stderr) taskLog += result.stderr.endsWith("\n") ? result.stderr : result.stderr + "\n";
      await refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
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

<main>
  <header>
    <h1>Lattice</h1>
    <p class="muted">Operator console</p>
  </header>

  <section>
    <div class="row">
      <h2>Sessions</h2>
      <button onclick={() => refresh()} disabled={loading}>Refresh</button>
    </div>
    {#if sessions.length === 0}
      <p class="muted">No sessions yet. Start an agent against the HTTPS channel.</p>
    {:else}
      <table>
        <thead>
          <tr>
            <th>Agent</th>
            <th>User@Host</th>
            <th>OS</th>
            <th>Last seen</th>
          </tr>
        </thead>
        <tbody>
          {#each sessions as s}
            <tr
              class:selected={selectedAgent === s.agentId}
              onclick={() => (selectedAgent = s.agentId)}
            >
              <td class="mono">{s.agentId}</td>
              <td>{s.username}@{s.hostname}</td>
              <td>{s.os}/{s.arch}</td>
              <td class="mono">{s.lastSeen ?? ""}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>

  <section>
    <h2>Task</h2>
    <div class="form">
      <label>
        Agent ID
        <input class="mono" bind:value={selectedAgent} placeholder="agent uuid" />
      </label>
      <label>
        Command
        <input bind:value={command} placeholder="whoami" />
      </label>
      <label>
        Args
        <input bind:value={argsText} placeholder="optional space-separated args" />
      </label>
      <button onclick={() => runTask()} disabled={loading || !selectedAgent}>Run</button>
    </div>
    {#if taskLog}
      <pre class="mono log">{taskLog}</pre>
    {/if}
  </section>

  {#if error}
    <p class="error">{error}</p>
  {/if}
</main>

<style>
  main {
    max-width: 960px;
    margin: 0 auto;
    padding: 1.5rem;
    display: grid;
    gap: 1.5rem;
  }

  header h1 {
    margin: 0;
  }

  header p {
    margin: 0.25rem 0 0;
  }

  section {
    background: #151821;
    border: 1px solid #2a2f3a;
    border-radius: 10px;
    padding: 1rem;
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }

  h2 {
    margin: 0 0 0.75rem;
    font-size: 1.05rem;
  }

  tr.selected {
    background: #1e293b;
  }

  tr {
    cursor: pointer;
  }

  .form {
    display: grid;
    gap: 0.75rem;
  }

  label {
    display: grid;
    gap: 0.35rem;
    font-size: 0.9rem;
    color: #9aa3b2;
  }

  .log {
    margin: 1rem 0 0;
    padding: 0.75rem;
    background: #0f1115;
    border-radius: 8px;
    overflow: auto;
    white-space: pre-wrap;
  }
</style>
