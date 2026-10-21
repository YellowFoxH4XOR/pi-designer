package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/akki/pidesigner/internal/model"
)

// AppendRunEvent commits the journal and its projection atomically. A slow SSE
// subscriber cannot drop events or block the RPC reader through a network write.
func (s *Store) AppendRunEvent(e *model.AgentEvent, a *model.Activity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if e.Version == 0 {
		e.Version = 1
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	result, err := tx.Exec(`INSERT INTO run_events(run_id,body) VALUES(?,?)`, e.RunID, string(payload))
	if err != nil {
		return err
	}
	seq, err := result.LastInsertId()
	if err != nil {
		return err
	}
	e.Seq = seq
	next := *a
	// Apply mutates tool slices; failure must not leave the caller half committed.
	next.TextBlocks = map[int]string{}
	for k, v := range a.TextBlocks {
		next.TextBlocks[k] = v
	}
	next.Tools = append([]model.ToolActivity{}, a.Tools...)
	next.Renders = append([]model.Render{}, a.Renders...)
	next.Critiques = append([]model.Critique{}, a.Critiques...)
	next.Apply(*e)
	body, err := json.Marshal(next)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO run_activity(run_id,body) VALUES(?,?) ON CONFLICT(run_id) DO UPDATE SET body=excluded.body`, e.RunID, string(body))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	*a = next
	return nil
}

func (s *Store) RunActivity(id string) (model.Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body string
	var a model.Activity
	err := s.db.QueryRow(`SELECT body FROM run_activity WHERE run_id=?`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	err = json.Unmarshal([]byte(body), &a)
	return a, err
}

// RunEvents pages the journal strictly after a cursor. Seq is shared across
// runs, but every query is run-scoped so cursors cannot cross project data.
func (s *Store) RunEvents(id string, after int64, limit int) ([]model.AgentEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows, err := s.db.Query(`SELECT seq,body FROM run_events WHERE run_id=? AND seq>? ORDER BY seq LIMIT ?`, id, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AgentEvent{}
	for rows.Next() {
		var e model.AgentEvent
		var body string
		var seq int64
		if err = rows.Scan(&seq, &body); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(body), &e); err != nil {
			return nil, fmt.Errorf("decode event: %w", err)
		}
		e.Seq = seq
		out = append(out, e)
	}
	return out, rows.Err()
}

// LatestRuns returns one latest run per project, plus older running runs if a
// future implementation permits them. It also finds editor/MCP-created work.
func (s *Store) LatestRuns() ([]model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT ` + runColumns + ` FROM runs WHERE status='running' OR id IN (SELECT id FROM (SELECT id,started_at,ROW_NUMBER() OVER(PARTITION BY project_id ORDER BY started_at DESC,rowid DESC) n FROM runs) WHERE n=1 ORDER BY started_at DESC LIMIT 100) ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Run{}
	for rows.Next() {
		r, err := scanRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
