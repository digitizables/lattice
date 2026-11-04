package store

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store is the Postgres-backed persistence layer.
type Store struct {
	pool *pgxpool.Pool
}

// Agent is a checked-in implant row (maps closely to operator Session).
type Agent struct {
	ID           string
	Hostname     string
	Username     string
	OS           string
	Arch         string
	SleepSeconds uint64
	LastSeen     time.Time
	CreatedAt    time.Time
}

// Task is queued or finished work for an agent.
type Task struct {
	ID        string
	AgentID   string
	Name      string
	Args      []string
	Status    string
	ExitCode  *int32
	Stdout    string
	Stderr    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New opens a pool, runs migrations, and returns a Store.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	s := &Store{pool: pool}
	if err := s.migrate(); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(s.pool)
	if err := goose.Up(db, "migrations"); err != nil {
		_ = db.Close()
		return fmt.Errorf("goose up: %w", err)
	}
	return db.Close()
}

// Close releases the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// UpsertAgent inserts or refreshes an agent on check-in.
func (s *Store) UpsertAgent(ctx context.Context, a Agent) error {
	const q = `
INSERT INTO agents (id, hostname, username, os, arch, sleep_seconds, last_seen)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
ON CONFLICT (id) DO UPDATE SET
  hostname = EXCLUDED.hostname,
  username = EXCLUDED.username,
  os = EXCLUDED.os,
  arch = EXCLUDED.arch,
  sleep_seconds = EXCLUDED.sleep_seconds,
  last_seen = NOW()`
	_, err := s.pool.Exec(ctx, q,
		a.ID, a.Hostname, a.Username, a.OS, a.Arch, int64(a.SleepSeconds),
	)
	return err
}

// DeleteAgent removes an agent and cascaded tasks.
func (s *Store) DeleteAgent(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent %s not found", id)
	}
	return nil
}

// ListAgents returns all agents, newest check-in first.
func (s *Store) ListAgents(ctx context.Context) ([]Agent, error) {
	const q = `
SELECT id, hostname, username, os, arch, sleep_seconds, last_seen, created_at
FROM agents
ORDER BY last_seen DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		var sleep int64
		if err := rows.Scan(
			&a.ID, &a.Hostname, &a.Username, &a.OS, &a.Arch,
			&sleep, &a.LastSeen, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		a.SleepSeconds = uint64(sleep)
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAgent returns one agent by id.
func (s *Store) GetAgent(ctx context.Context, id string) (Agent, error) {
	const q = `
SELECT id, hostname, username, os, arch, sleep_seconds, last_seen, created_at
FROM agents WHERE id = $1`
	var a Agent
	var sleep int64
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.Hostname, &a.Username, &a.OS, &a.Arch,
		&sleep, &a.LastSeen, &a.CreatedAt,
	)
	if err != nil {
		return Agent{}, err
	}
	a.SleepSeconds = uint64(sleep)
	return a, nil
}

// GetTask returns one task by id.
func (s *Store) GetTask(ctx context.Context, id string) (Task, error) {
	const q = `
SELECT id, agent_id, name, args, status, exit_code, stdout, stderr, created_at, updated_at
FROM tasks WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	var t Task
	if err := row.Scan(
		&t.ID, &t.AgentID, &t.Name, &t.Args, &t.Status,
		&t.ExitCode, &t.Stdout, &t.Stderr, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return Task{}, err
	}
	return t, nil
}

// CreateTask inserts a queued task.
func (s *Store) CreateTask(ctx context.Context, t Task) error {
	args := t.Args
	if args == nil {
		args = []string{}
	}
	const q = `
INSERT INTO tasks (id, agent_id, name, args, status)
VALUES ($1, $2, $3, $4, 'queued')`
	_, err := s.pool.Exec(ctx, q, t.ID, t.AgentID, t.Name, args)
	return err
}

// DequeueTasks marks up to limit queued tasks as sent and returns them.
func (s *Store) DequeueTasks(ctx context.Context, agentID string, limit int) ([]Task, error) {
	const q = `
WITH next AS (
  SELECT id FROM tasks
  WHERE agent_id = $1 AND status = 'queued'
  ORDER BY created_at
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
UPDATE tasks t
SET status = 'sent', updated_at = NOW()
FROM next
WHERE t.id = next.id
RETURNING t.id, t.agent_id, t.name, t.args, t.status, t.exit_code, t.stdout, t.stderr, t.created_at, t.updated_at`

	rows, err := s.pool.Query(ctx, q, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

// CompleteTask stores a result from the agent.
func (s *Store) CompleteTask(ctx context.Context, taskID, agentID string, exitCode int32, stdout, stderr string) error {
	const q = `
UPDATE tasks
SET status = 'completed',
    exit_code = $3,
    stdout = $4,
    stderr = $5,
    updated_at = NOW()
WHERE id = $1 AND agent_id = $2 AND status IN ('queued', 'sent')`
	tag, err := s.pool.Exec(ctx, q, taskID, agentID, exitCode, stdout, stderr)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task %s not found or not updatable for agent %s", taskID, agentID)
	}
	return nil
}

type scannable interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanTasks(rows scannable) ([]Task, error) {
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(
			&t.ID, &t.AgentID, &t.Name, &t.Args, &t.Status,
			&t.ExitCode, &t.Stdout, &t.Stderr, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
