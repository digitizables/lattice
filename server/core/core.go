package core

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
	"github.com/digitizables/lattice/server/store"
)

// Core is the teamserver domain logic on top of the store.
type Core struct {
	store *store.Store
	bus   *Bus
}

// New wraps a store.
func New(s *store.Store) *Core {
	return &Core{store: s, bus: newBus()}
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

	err := c.store.UpsertAgent(ctx, store.Agent{
		ID:           agentID,
		Hostname:     in.GetHostname(),
		Username:     in.GetUsername(),
		OS:           in.GetOs(),
		Arch:         in.GetArch(),
		SleepSeconds: in.GetSleepSeconds(),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert agent: %w", err)
	}

	agent, err := c.store.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("load agent: %w", err)
	}
	c.bus.publish(Event{Session: &agent})

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
	return out, nil
}

// HandleTaskResult records an agent task result.
func (c *Core) HandleTaskResult(ctx context.Context, in *agentv1.AgentTaskResult) (*agentv1.AgentTaskResultResponse, error) {
	err := c.store.CompleteTask(
		ctx,
		in.GetTaskId(),
		in.GetAgentId(),
		in.GetExitCode(),
		in.GetStdout(),
		in.GetStderr(),
	)
	if err != nil {
		return nil, err
	}
	task, err := c.store.GetTask(ctx, in.GetTaskId())
	if err != nil {
		return nil, err
	}
	c.bus.publish(Event{Task: &task})
	return &agentv1.AgentTaskResultResponse{Accepted: true}, nil
}

// ListSessions returns agents as operator-facing session rows.
func (c *Core) ListSessions(ctx context.Context) ([]store.Agent, error) {
	return c.store.ListAgents(ctx)
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
