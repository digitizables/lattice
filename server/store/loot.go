package store

import (
	"context"
	"fmt"
	"strings"
)

// InsertLoot records loot metadata (bytes live on disk at StoragePath).
func (s *Store) InsertLoot(ctx context.Context, l Loot) error {
	if l.ID == "" {
		l.ID = NewID()
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO loot (id, agent_id, task_id, name, remote_path, content_type, size_bytes, sha256, storage_path)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.ID, l.AgentID, l.TaskID, l.Name, l.RemotePath, l.ContentType,
		int64(l.SizeBytes), l.SHA256, l.StoragePath,
	)
	return err
}

// GetLoot returns one loot row.
func (s *Store) GetLoot(ctx context.Context, id string) (Loot, error) {
	const q = `
SELECT id, agent_id, task_id, name, remote_path, content_type, size_bytes, sha256, storage_path, created_at
FROM loot WHERE id = $1`
	var l Loot
	var size int64
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&l.ID, &l.AgentID, &l.TaskID, &l.Name, &l.RemotePath, &l.ContentType,
		&size, &l.SHA256, &l.StoragePath, &l.CreatedAt,
	)
	if err != nil {
		return Loot{}, err
	}
	l.SizeBytes = uint64(size)
	return l, nil
}

// DeleteLoot removes a loot row.
func (s *Store) DeleteLoot(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM loot WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("loot %s not found", id)
	}
	return nil
}

// LootPage is a page of loot items.
type LootPage struct {
	Items         []Loot
	NextPageToken string
	TotalCount    int32
}

// ListLootPage returns loot newest-first, optionally filtered by agent.
func (s *Store) ListLootPage(ctx context.Context, agentID string, pageSize int32, pageToken string) (LootPage, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	filter := []string{"TRUE"}
	args := []any{}
	argn := 1
	if strings.TrimSpace(agentID) != "" {
		filter = append(filter, fmt.Sprintf("agent_id = $%d", argn))
		args = append(args, agentID)
		argn++
	}
	var total int32
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM loot WHERE `+strings.Join(filter, " AND "), args...).Scan(&total); err != nil {
		return LootPage{}, err
	}

	listFilter := append([]string{}, filter...)
	listArgs := append([]any{}, args...)
	listArgn := argn
	if pageToken != "" {
		ts, id, err := decodeAgentCursor(pageToken)
		if err != nil {
			return LootPage{}, fmt.Errorf("invalid page_token: %w", err)
		}
		listFilter = append(listFilter, fmt.Sprintf("(created_at, id) < ($%d::timestamptz, $%d)", listArgn, listArgn+1))
		listArgs = append(listArgs, ts, id)
		listArgn += 2
	}
	listArgs = append(listArgs, pageSize)
	q := `
SELECT id, agent_id, task_id, name, remote_path, content_type, size_bytes, sha256, storage_path, created_at
FROM loot
WHERE ` + strings.Join(listFilter, " AND ") + `
ORDER BY created_at DESC, id DESC
LIMIT $` + fmt.Sprint(listArgn)

	rows, err := s.pool.Query(ctx, q, listArgs...)
	if err != nil {
		return LootPage{}, err
	}
	defer rows.Close()
	var items []Loot
	for rows.Next() {
		var l Loot
		var size int64
		if err := rows.Scan(
			&l.ID, &l.AgentID, &l.TaskID, &l.Name, &l.RemotePath, &l.ContentType,
			&size, &l.SHA256, &l.StoragePath, &l.CreatedAt,
		); err != nil {
			return LootPage{}, err
		}
		l.SizeBytes = uint64(size)
		items = append(items, l)
	}
	page := LootPage{Items: items, TotalCount: total}
	if int32(len(items)) == pageSize && len(items) > 0 {
		last := items[len(items)-1]
		page.NextPageToken = encodeAgentCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}
