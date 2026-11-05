package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// NewID returns a random UUID string.
func NewID() string { return uuid.NewString() }

// CountOperators returns how many operator accounts exist.
func (s *Store) CountOperators(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM operators`).Scan(&n)
	return n, err
}

// CreateOperator inserts a new operator row.
func (s *Store) CreateOperator(ctx context.Context, op Operator) error {
	if op.ID == "" {
		op.ID = NewID()
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO operators (id, username, password_hash)
VALUES ($1, $2, $3)`, op.ID, op.Username, op.PasswordHash)
	return err
}

// GetOperatorByUsername loads an operator by username (case-insensitive).
func (s *Store) GetOperatorByUsername(ctx context.Context, username string) (Operator, error) {
	const q = `
SELECT id, username, password_hash, created_at
FROM operators WHERE lower(username) = lower($1)`
	var op Operator
	err := s.pool.QueryRow(ctx, q, strings.TrimSpace(username)).Scan(
		&op.ID, &op.Username, &op.PasswordHash, &op.CreatedAt,
	)
	if err != nil {
		return Operator{}, err
	}
	return op, nil
}

// EnsureStringSlice returns a non-nil empty slice.
func EnsureStringSlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// SetAgentNote updates the operator note for a session.
func (s *Store) SetAgentNote(ctx context.Context, id, note string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE agents SET note = $2 WHERE id = $1`, id, note)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent %s not found", id)
	}
	return nil
}

// SetAgentDesiredSleep stores sleep/jitter to push on the next check-in.
func (s *Store) SetAgentDesiredSleep(ctx context.Context, id string, sleepSeconds uint64, jitterPercent uint32) error {
	if jitterPercent > 100 {
		jitterPercent = 100
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE agents SET
  desired_sleep_seconds = $2,
  desired_jitter_percent = $3,
  has_desired_sleep = TRUE
WHERE id = $1`, id, int64(sleepSeconds), int32(jitterPercent))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent %s not found", id)
	}
	return nil
}

// ClearAgentDesiredSleep clears the pending sleep directive after the agent applies it.
func (s *Store) ClearAgentDesiredSleep(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE agents SET
  has_desired_sleep = FALSE,
  desired_sleep_seconds = NULL,
  desired_jitter_percent = NULL
WHERE id = $1`, id)
	return err
}

// TouchAgentSleep writes the agent's reported sleep/jitter after apply.
func (s *Store) TouchAgentSleep(ctx context.Context, id string, sleepSeconds uint64, jitterPercent uint32) error {
	_, err := s.pool.Exec(ctx, `
UPDATE agents SET sleep_seconds = $2, jitter_percent = $3 WHERE id = $1`,
		id, int64(sleepSeconds), int32(jitterPercent))
	return err
}

// scanAgentRow is shared SELECT list for agent rows after migration 00004.
const agentSelectCols = `
id, hostname, username, os, arch, sleep_seconds, COALESCE(jitter_percent,0), cwd,
COALESCE(note,''), COALESCE(features,'{}'), COALESCE(ops,'{}'),
desired_sleep_seconds, desired_jitter_percent, COALESCE(has_desired_sleep,FALSE),
last_seen, created_at`

func scanAgent(scanner interface {
	Scan(dest ...any) error
}) (Agent, error) {
	var a Agent
	var sleep int64
	var jitter int32
	var desiredSleep *int64
	var desiredJitter *int32
	err := scanner.Scan(
		&a.ID, &a.Hostname, &a.Username, &a.OS, &a.Arch,
		&sleep, &jitter, &a.Cwd, &a.Note, &a.Features, &a.Ops,
		&desiredSleep, &desiredJitter, &a.HasDesiredSleep,
		&a.LastSeen, &a.CreatedAt,
	)
	if err != nil {
		return Agent{}, err
	}
	a.SleepSeconds = uint64(sleep)
	if jitter < 0 {
		jitter = 0
	}
	a.JitterPercent = uint32(jitter)
	a.Features = EnsureStringSlice(a.Features)
	a.Ops = EnsureStringSlice(a.Ops)
	if desiredSleep != nil {
		a.DesiredSleepSeconds = uint64(*desiredSleep)
	}
	if desiredJitter != nil && *desiredJitter >= 0 {
		a.DesiredJitterPercent = uint32(*desiredJitter)
	}
	return a, nil
}
