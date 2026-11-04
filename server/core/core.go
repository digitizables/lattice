package core

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
	"github.com/digitizables/lattice/server/store"
	"github.com/digitizables/lattice/server/updates"
)

// Core is the teamserver domain logic on top of the store.
type Core struct {
	store   *store.Store
	bus     *Bus
	updates *updates.Store
}

// New wraps a store and optional update catalog.
func New(s *store.Store, u *updates.Store) *Core {
	return &Core{store: s, bus: newBus(), updates: u}
}

// SubscribeEvents returns a live event feed for operators.
func (c *Core) SubscribeEvents(buffer int) (<-chan Event, func()) {
	return c.bus.Subscribe(buffer)
}

// HandleCheckIn upserts the agent and returns any queued tasks.
func (c *Core) HandleCheckIn(ctx context.Context, in *agentv1.AgentCheckIn) (*agentv1.AgentCheckInResponse, error) {
	agentID := in.GetAgentId()
	if agentID == "" {
		agentID = uuid.NewString()
	}

	created, err := c.store.UpsertAgent(ctx, store.Agent{
		ID:           agentID,
		Hostname:     in.GetHostname(),
		Username:     in.GetUsername(),
		OS:           in.GetOs(),
		Arch:         in.GetArch(),
		SleepSeconds: in.GetSleepSeconds(),
		Cwd:          in.GetCwd(),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert agent: %w", err)
	}

	// Only broadcast brand-new sessions. Routine beacons must not fan out to every operator.
	if created {
		agent, err := c.store.GetAgent(ctx, agentID)
		if err != nil {
			return nil, fmt.Errorf("load agent: %w", err)
		}
		c.bus.publish(Event{Session: &agent})
	}

	tasks, err := c.store.DequeueTasks(ctx, agentID, 16)
	if err != nil {
		return nil, fmt.Errorf("dequeue tasks: %w", err)
	}
	for i := range tasks {
		t := tasks[i]
		c.bus.publish(Event{Task: &t})
	}

	out := &agentv1.AgentCheckInResponse{
		AgentId: agentID,
		Tasks:   make([]*agentv1.Task, 0, len(tasks)),
	}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, &agentv1.Task{
			TaskId: t.ID,
			Name:   t.Name,
			Args:   t.Args,
		})
	}
	if info, ok := c.UpdateHint(in.GetOs(), in.GetArch(), in.GetVersion()); ok {
		out.Update = info
	}
	return out, nil
}

// UpdateHint returns update metadata when a newer build is published.
func (c *Core) UpdateHint(osName, arch, current string) (*agentv1.UpdateInfo, bool) {
	if c.updates == nil {
		return nil, false
	}
	art, ok, err := c.updates.Latest(osName, arch)
	if err != nil || !ok || !updates.NewerThan(art.Version, current) {
		return nil, false
	}
	return &agentv1.UpdateInfo{
		Version: art.Version,
		Os:      art.OS,
		Arch:    art.Arch,
		Sha256:  art.SHA256,
		Size:    art.Size,
	}, true
}

// CheckUpdate is the agent update-check RPC.
func (c *Core) CheckUpdate(osName, arch, current string) (bool, *agentv1.UpdateInfo, error) {
	info, ok := c.UpdateHint(osName, arch, current)
	return ok, info, nil
}

// DownloadUpdate returns published binary bytes for the agent.
func (c *Core) DownloadUpdate(osName, arch, version string) (*agentv1.UpdateInfo, []byte, error) {
	if c.updates == nil {
		return nil, nil, fmt.Errorf("update service not configured")
	}
	art, data, err := c.updates.Get(osName, arch, version)
	if err != nil {
		return nil, nil, err
	}
	return &agentv1.UpdateInfo{
		Version: art.Version,
		Os:      art.OS,
		Arch:    art.Arch,
		Sha256:  art.SHA256,
		Size:    art.Size,
	}, data, nil
}

// PublishUpdate stores an operator-uploaded agent build.
func (c *Core) PublishUpdate(version, osName, arch string, data []byte) (updates.Artifact, error) {
	if c.updates == nil {
		return updates.Artifact{}, fmt.Errorf("update service not configured")
	}
	return c.updates.Publish(version, osName, arch, data)
}

// ListUpdates returns published agent builds.
func (c *Core) ListUpdates() ([]updates.Artifact, error) {
	if c.updates == nil {
		return nil, nil
	}
	return c.updates.List()
}

// HandleTaskResult records an agent task result.
func (c *Core) HandleTaskResult(ctx context.Context, in *agentv1.AgentTaskResult) (*agentv1.AgentTaskResultResponse, error) {
	status := "completed"
	if in.GetExitCode() == 130 {
		status = "cancelled"
	}
	err := c.store.FinishTask(
		ctx,
		in.GetTaskId(),
		in.GetAgentId(),
		status,
		in.GetExitCode(),
		in.GetStdout(),
		in.GetStderr(),
	)
	if err != nil {
		return nil, err
	}
	if cwd := in.GetCwd(); cwd != "" {
		_ = c.store.UpdateAgentCwd(ctx, in.GetAgentId(), cwd)
	}
	task, err := c.store.GetTask(ctx, in.GetTaskId())
	if err != nil {
		return nil, err
	}
	c.bus.publish(Event{Task: &task})
	return &agentv1.AgentTaskResultResponse{Accepted: true}, nil
}

// MarkTaskRunning records that the agent started executing a task.
func (c *Core) MarkTaskRunning(ctx context.Context, agentID, taskID string) (cancelled bool, err error) {
	cancelled, err = c.store.MarkTaskRunning(ctx, taskID, agentID)
	if err != nil {
		return false, err
	}
	if task, getErr := c.store.GetTask(ctx, taskID); getErr == nil {
		c.bus.publish(Event{Task: &task})
	}
	return cancelled, nil
}

// IsTaskCancelled reports cancel state for an in-flight agent command.
func (c *Core) IsTaskCancelled(ctx context.Context, agentID, taskID string) (bool, error) {
	return c.store.IsTaskCancelled(ctx, taskID, agentID)
}

// CancelTask aborts a queued/sent/running task (operator Ctrl+C).
func (c *Core) CancelTask(ctx context.Context, taskID string) (bool, error) {
	if taskID == "" {
		return false, fmt.Errorf("task_id is required")
	}
	ok, err := c.store.CancelTask(ctx, taskID)
	if err != nil {
		return false, err
	}
	if ok {
		if task, getErr := c.store.GetTask(ctx, taskID); getErr == nil {
			c.bus.publish(Event{Task: &task})
		}
	}
	return ok, nil
}

// ListSessions returns a page of agents.
func (c *Core) ListSessions(ctx context.Context, pageSize int32, pageToken, query string) (store.AgentPage, error) {
	return c.store.ListAgentsPage(ctx, pageSize, pageToken, query)
}

// DeleteSession removes a session/agent row.
func (c *Core) DeleteSession(ctx context.Context, agentID string) error {
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	return c.store.DeleteAgent(ctx, agentID)
}

// GetTask returns one task by id.
func (c *Core) GetTask(ctx context.Context, taskID string) (store.Task, error) {
	return c.store.GetTask(ctx, taskID)
}

// SubmitTask queues a task for an agent and returns the new task id.
func (c *Core) SubmitTask(ctx context.Context, agentID, name string, args []string) (string, error) {
	if _, err := c.store.GetAgent(ctx, agentID); err != nil {
		return "", fmt.Errorf("unknown agent %s: %w", agentID, err)
	}
	taskID := uuid.NewString()
	err := c.store.CreateTask(ctx, store.Task{
		ID:      taskID,
		AgentID: agentID,
		Name:    name,
		Args:    args,
	})
	if err != nil {
		return "", err
	}
	task, err := c.store.GetTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	c.bus.publish(Event{Task: &task})
	return taskID, nil
}
