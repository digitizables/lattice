package store

import (
	"context"
	"encoding/base64"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	Cwd          string
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
// created is true when this was the first insert for the id.
func (s *Store) UpsertAgent(ctx context.Context, a Agent) (created bool, err error) {
	_, getErr := s.GetAgent(ctx, a.ID)
	switch {
	case getErr == nil:
		created = false
	case errors.Is(getErr, pgx.ErrNoRows):
		created = true
	default:
		return false, getErr
	}
	const q = `
INSERT INTO agents (id, hostname, username, os, arch, sleep_seconds, cwd, last_seen)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
ON CONFLICT (id) DO UPDATE SET
  hostname = EXCLUDED.hostname,
  username = EXCLUDED.username,
  os = EXCLUDED.os,
  arch = EXCLUDED.arch,
  sleep_seconds = EXCLUDED.sleep_seconds,
  cwd = EXCLUDED.cwd,
  last_seen = NOW()`
	_, err = s.pool.Exec(ctx, q,
		a.ID, a.Hostname, a.Username, a.OS, a.Arch, int64(a.SleepSeconds), a.Cwd,
	)
	return created, err
}

// UpdateAgentCwd stores the agent's working directory.
func (s *Store) UpdateAgentCwd(ctx context.Context, id, cwd string) error {
	_, err := s.pool.Exec(ctx, `UPDATE agents SET cwd = $2 WHERE id = $1`, id, cwd)
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

// DeleteAgentsByHostname removes every agent whose hostname matches (case-insensitive).
// Used to purge leftover command-center host sessions when local agents are denied.
func (s *Store) DeleteAgentsByHostname(ctx context.Context, hostname string) (int64, error) {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM agents WHERE lower(hostname) = lower($1)`, hostname)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// AgentPage is one page of agents plus pagination metadata.
type AgentPage struct {
	Agents        []Agent
	NextPageToken string
	TotalCount    int32
	ActiveCount   int32
}

// ActiveWindow is how recent last_seen must be to count as online.
const ActiveWindow = 30 * time.Second

// ListAgentsPage returns a cursor page of agents ordered by last_seen DESC, id DESC.
func (s *Store) ListAgentsPage(ctx context.Context, pageSize int32, pageToken, query string) (AgentPage, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	filterArgs := []any{}
	filterWhere := []string{"TRUE"}
	argn := 1

	q := strings.TrimSpace(query)
	if q != "" {
		pattern := "%" + escapeLike(q) + "%"
		filterWhere = append(filterWhere, fmt.Sprintf(
			"(id ILIKE $%d ESCAPE '\\' OR hostname ILIKE $%d ESCAPE '\\' OR username ILIKE $%d ESCAPE '\\' OR os ILIKE $%d ESCAPE '\\' OR arch ILIKE $%d ESCAPE '\\')",
			argn, argn, argn, argn, argn,
		))
		filterArgs = append(filterArgs, pattern)
		argn++
	}

	countQ := `SELECT COUNT(*) FROM agents WHERE ` + strings.Join(filterWhere, " AND ")
	var total int32
	if err := s.pool.QueryRow(ctx, countQ, filterArgs...).Scan(&total); err != nil {
		return AgentPage{}, err
	}

	activeWhere := append(append([]string{}, filterWhere...), "last_seen > NOW() - INTERVAL '30 seconds'")
	activeQ := `SELECT COUNT(*) FROM agents WHERE ` + strings.Join(activeWhere, " AND ")
	var active int32
	if err := s.pool.QueryRow(ctx, activeQ, filterArgs...).Scan(&active); err != nil {
		return AgentPage{}, err
	}

	listWhere := append([]string{}, filterWhere...)
	listArgs := append([]any{}, filterArgs...)
	listArgn := argn
	if pageToken != "" {
		ts, id, err := decodeAgentCursor(pageToken)
		if err != nil {
			return AgentPage{}, fmt.Errorf("invalid page_token: %w", err)
		}
		listWhere = append(listWhere, fmt.Sprintf("(last_seen, id) < ($%d::timestamptz, $%d)", listArgn, listArgn+1))
		listArgs = append(listArgs, ts, id)
		listArgn += 2
	}

	limitPlaceholder := fmt.Sprintf("$%d", listArgn)
	listArgs = append(listArgs, pageSize)
	listQ := `
SELECT id, hostname, username, os, arch, sleep_seconds, cwd, last_seen, created_at
FROM agents
WHERE ` + strings.Join(listWhere, " AND ") + `
ORDER BY last_seen DESC, id DESC
LIMIT ` + limitPlaceholder

	rows, err := s.pool.Query(ctx, listQ, listArgs...)
	if err != nil {
		return AgentPage{}, err
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		var sleep int64
		if err := rows.Scan(
			&a.ID, &a.Hostname, &a.Username, &a.OS, &a.Arch,
			&sleep, &a.Cwd, &a.LastSeen, &a.CreatedAt,
		); err != nil {
			return AgentPage{}, err
		}
		a.SleepSeconds = uint64(sleep)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return AgentPage{}, err
	}

	page := AgentPage{Agents: out, TotalCount: total, ActiveCount: active}
	if int32(len(out)) == pageSize && len(out) > 0 {
		last := out[len(out)-1]
		page.NextPageToken = encodeAgentCursor(last.LastSeen, last.ID)
	}
	return page, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func encodeAgentCursor(ts time.Time, id string) string {
	raw := ts.UTC().Format(time.RFC3339Nano) + "\x00" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeAgentCursor(token string) (time.Time, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(b), "\x00", 2)
	if len(parts) != 2 || parts[1] == "" {
		return time.Time{}, "", fmt.Errorf("malformed cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return ts, parts[1], nil
}

// GetAgent returns one agent by id.
func (s *Store) GetAgent(ctx context.Context, id string) (Agent, error) {
	const q = `
SELECT id, hostname, username, os, arch, sleep_seconds, cwd, last_seen, created_at
FROM agents WHERE id = $1`
	var a Agent
	var sleep int64
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.Hostname, &a.Username, &a.OS, &a.Arch,
		&sleep, &a.Cwd, &a.LastSeen, &a.CreatedAt,
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

// MarkTaskRunning moves a sent task to running. Returns cancelled if operator already aborted.
func (s *Store) MarkTaskRunning(ctx context.Context, taskID, agentID string) (cancelled bool, err error) {
	var status string
	err = s.pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1 AND agent_id = $2`, taskID, agentID).Scan(&status)
	if err != nil {
		return false, err
	}
	if status == "cancelled" {
		return true, nil
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE tasks SET status = 'running', updated_at = NOW()
WHERE id = $1 AND agent_id = $2 AND status IN ('queued', 'sent')`, taskID, agentID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		err = s.pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1 AND agent_id = $2`, taskID, agentID).Scan(&status)
		if err != nil {
			return false, err
		}
		return status == "cancelled", nil
	}
	return false, nil
}

// CancelTask marks a non-terminal task as cancelled.
func (s *Store) CancelTask(ctx context.Context, taskID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE tasks SET status = 'cancelled', updated_at = NOW()
WHERE id = $1 AND status IN ('queued', 'sent', 'running')`, taskID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// IsTaskCancelled reports whether the task was cancelled.
func (s *Store) IsTaskCancelled(ctx context.Context, taskID, agentID string) (bool, error) {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1 AND agent_id = $2`, taskID, agentID).Scan(&status)
	if err != nil {
		return false, err
	}
	return status == "cancelled", nil
}

// CompleteTask stores a result from the agent.
func (s *Store) CompleteTask(ctx context.Context, taskID, agentID string, exitCode int32, stdout, stderr string) error {
	return s.FinishTask(ctx, taskID, agentID, "completed", exitCode, stdout, stderr)
}

// FinishTask writes a terminal task status (completed, failed, cancelled).
func (s *Store) FinishTask(ctx context.Context, taskID, agentID, status string, exitCode int32, stdout, stderr string) error {
	const q = `
UPDATE tasks
SET status = $3,
    exit_code = $4,
    stdout = $5,
    stderr = $6,
    updated_at = NOW()
WHERE id = $1 AND agent_id = $2 AND status IN ('queued', 'sent', 'running', 'cancelled')`
	tag, err := s.pool.Exec(ctx, q, taskID, agentID, status, exitCode, stdout, stderr)
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
