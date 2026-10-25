// Package store persists projects, conversations, messages and runs in SQLite.
//
// The database is the daemon's source of truth for chat history; project files
// live on disk and are addressed by relative path.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/akki/pidesigner/internal/model"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a lookup misses.
var ErrNotFound = errors.New("not found")

// Store wraps the SQLite handle.
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite serializes writes; a small pool avoids lock contention.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			dir TEXT NOT NULL,
			design_system_id TEXT,
			skill_id TEXT,
			entry_file TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			title TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			conversation_id TEXT,
			status TEXT NOT NULL,
			prompt TEXT,
			error TEXT,
			started_at INTEGER NOT NULL,
			ended_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS run_events (seq INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE, body TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_run_events ON run_events(run_id,seq)`,
		`CREATE TABLE IF NOT EXISTS run_activity (run_id TEXT PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE, body TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS run_summary (run_id TEXT PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE, body TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_conv_project ON conversations(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_msg_conv ON messages(conversation_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_run_project ON runs(project_id, started_at)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// A run's latest plan, preview renders, and critique rounds, as JSON arrays.
	for _, col := range []string{"todos", "renders", "critiques"} {
		if err := s.addColumn("runs", col, "TEXT"); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// A compact copy of the projection in its own table, so activity polling
	// neither decodes whole transcripts nor walks their overflow pages.
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO run_summary(run_id,body) SELECT run_id,json_remove(body,'$.text','$.textBlocks','$.messageText','$.prompt','$.tools','$.request','$.todos','$.renders','$.critiques') FROM run_activity WHERE json_valid(body)`); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	// web-artifacts-builder is a catalogue stub without a workflow; the
	// Prototype workflow is the web-prototype skill.
	if _, err := s.db.Exec(`UPDATE projects SET skill_id='web-prototype' WHERE skill_id='web-artifacts-builder'`); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// addColumn adds a column unless the table already has it.
func (s *Store) addColumn(table, column, typ string) error {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, typ))
	return err
}

// ---------------------------------------------------------------- projects

// CreateProject inserts a project row.
func (s *Store) CreateProject(p model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO projects (id,name,slug,dir,design_system_id,skill_id,entry_file,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Slug, p.Dir, p.DesignSystemID, p.SkillID, p.EntryFile,
		p.CreatedAt.UnixMilli(), p.UpdatedAt.UnixMilli(),
	)
	return err
}

// UpdateProject writes mutable project fields (design system, skill, entry file).
func (s *Store) UpdateProject(p model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`UPDATE projects SET name=?, design_system_id=?, skill_id=?, entry_file=?, updated_at=? WHERE id=?`,
		p.Name, p.DesignSystemID, p.SkillID, p.EntryFile, time.Now().UnixMilli(), p.ID,
	)
	return err
}

// TouchProject bumps the project updated_at timestamp.
func (s *Store) TouchProject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE projects SET updated_at=? WHERE id=?`, time.Now().UnixMilli(), id)
	return err
}

// GetProject loads one project by id or by slug/name substring.
func (s *Store) GetProject(ref string) (model.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT id,name,slug,dir,design_system_id,skill_id,entry_file,created_at,updated_at
		FROM projects WHERE id=?`, ref)
	p, err := scanProject(row)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.Project{}, err
	}
	// Fall back to a case-insensitive slug match, then a unique name prefix.
	row = s.db.QueryRow(`SELECT id,name,slug,dir,design_system_id,skill_id,entry_file,created_at,updated_at
		FROM projects WHERE lower(slug)=lower(?) OR lower(name)=lower(?) LIMIT 1`, ref, ref)
	p, err = scanProject(row)
	if err == nil {
		return p, nil
	}
	rows, err := s.db.Query(`SELECT id,name,slug,dir,design_system_id,skill_id,entry_file,created_at,updated_at
		FROM projects WHERE lower(slug) LIKE lower(?) ESCAPE '\' OR lower(name) LIKE lower(?) ESCAPE '\'`, likeContains(ref), likeContains(ref))
	if err != nil {
		return model.Project{}, err
	}
	defer rows.Close()
	var matches []model.Project
	for rows.Next() {
		m, err := scanProject(rows)
		if err != nil {
			return model.Project{}, err
		}
		matches = append(matches, m)
	}
	if len(matches) == 0 {
		return model.Project{}, ErrNotFound
	}
	if len(matches) > 1 {
		return model.Project{}, fmt.Errorf("ambiguous project reference %q (%d matches)", ref, len(matches))
	}
	return matches[0], nil
}

// likeContains builds a LIKE pattern matching ref literally anywhere, so
// "%" and "_" in a reference never act as wildcards.
func likeContains(ref string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(ref) + "%"
}

// SlugExists reports whether a project already uses slug (case-insensitive).
func (s *Store) SlugExists(slug string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	err := s.db.QueryRow(`SELECT count(*) FROM projects WHERE lower(slug)=lower(?)`, slug).Scan(&n)
	return n > 0, err
}

// ListProjects returns all projects, newest first.
func (s *Store) ListProjects() ([]model.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id,name,slug,dir,design_system_id,skill_id,entry_file,created_at,updated_at
		FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ProjectsStamp changes whenever a project is created, updated or deleted, so
// clients can poll it instead of the project list.
func (s *Store) ProjectsStamp() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var stamp string
	err := s.db.QueryRow(`SELECT count(*)||':'||COALESCE(sum(updated_at),0) FROM projects`).Scan(&stamp)
	return stamp, err
}

// DeleteProject removes a project row (files are removed by the caller).
func (s *Store) DeleteProject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM projects WHERE id=?`, id)
	return err
}

func scanProject(row interface{ Scan(...any) error }) (model.Project, error) {
	var p model.Project
	var created, updated int64
	var ds, skill, entry sql.NullString
	if err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.Dir, &ds, &skill, &entry, &created, &updated); err != nil {
		return model.Project{}, err
	}
	p.DesignSystemID, p.SkillID, p.EntryFile = ds.String, skill.String, entry.String
	p.CreatedAt = time.UnixMilli(created)
	p.UpdatedAt = time.UnixMilli(updated)
	return p, nil
}

// ----------------------------------------------------------- conversations

// CreateConversation inserts a conversation for a project.
func (s *Store) CreateConversation(c model.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO conversations (id,project_id,title,created_at,updated_at) VALUES (?,?,?,?,?)`,
		c.ID, c.ProjectID, c.Title, c.CreatedAt.UnixMilli(), c.UpdatedAt.UnixMilli())
	return err
}

// GetConversation loads one conversation.
func (s *Store) GetConversation(id string) (model.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT id,project_id,COALESCE(title,''),created_at,updated_at,
		(SELECT COUNT(*) FROM messages m WHERE m.conversation_id=conversations.id)
		FROM conversations WHERE id=?`, id)
	var c model.Conversation
	var created, updated int64
	if err := row.Scan(&c.ID, &c.ProjectID, &c.Title, &created, &updated, &c.MessageCnt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c, ErrNotFound
		}
		return c, err
	}
	c.CreatedAt = time.UnixMilli(created)
	c.UpdatedAt = time.UnixMilli(updated)
	return c, nil
}

// ListConversations returns a project's conversations, newest first.
func (s *Store) ListConversations(projectID string) ([]model.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT c.id,c.project_id,COALESCE(c.title,''),c.created_at,c.updated_at,
		(SELECT COUNT(*) FROM messages m WHERE m.conversation_id=c.id)
		FROM conversations c WHERE c.project_id=? ORDER BY c.updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Conversation
	for rows.Next() {
		var c model.Conversation
		var created, updated int64
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &created, &updated, &c.MessageCnt); err != nil {
			return nil, err
		}
		c.CreatedAt = time.UnixMilli(created)
		c.UpdatedAt = time.UnixMilli(updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

// --------------------------------------------------------------- messages

// AppendMessage stores a message.
func (s *Store) AppendMessage(m model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO messages (id,conversation_id,role,content,created_at) VALUES (?,?,?,?,?)`,
		m.ID, m.ConversationID, m.Role, m.Content, m.CreatedAt.UnixMilli()); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE conversations SET updated_at=? WHERE id=?`, time.Now().UnixMilli(), m.ConversationID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListMessages returns a conversation's messages in chronological order.
func (s *Store) ListMessages(conversationID string) ([]model.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id,conversation_id,role,content,created_at FROM messages
		WHERE conversation_id=? ORDER BY created_at ASC, rowid ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Message
	for rows.Next() {
		var m model.Message
		var created int64
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = time.UnixMilli(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ------------------------------------------------------------------ runs

// CreateRun inserts a run row.
func (s *Store) CreateRun(r model.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO runs (id,project_id,conversation_id,status,prompt,error,started_at,ended_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.ProjectID, r.ConversationID, r.Status, r.Prompt, r.Error,
		r.StartedAt.UnixMilli(), nil)
	return err
}

// CreateRunWithMessage inserts a run and its opening user message atomically.
// A failed transcript write must not leave a run that appears to be in progress.
func (s *Store) CreateRunWithMessage(r model.Run, message *model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO runs (id,project_id,conversation_id,status,prompt,error,started_at,ended_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.ProjectID, r.ConversationID, r.Status, r.Prompt, r.Error,
		r.StartedAt.UnixMilli(), nil); err != nil {
		return err
	}
	if message != nil {
		if _, err := tx.Exec(`INSERT INTO messages (id,conversation_id,role,content,created_at) VALUES (?,?,?,?,?)`,
			message.ID, message.ConversationID, message.Role, message.Content, message.CreatedAt.UnixMilli()); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE conversations SET updated_at=? WHERE id=?`, time.Now().UnixMilli(), message.ConversationID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FinishRun marks a run terminal.
func (s *Store) FinishRun(id, status, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE runs SET status=?, error=?, ended_at=? WHERE id=?`,
		status, errMsg, time.Now().UnixMilli(), id)
	return err
}

// SaveRunWork records a run's latest plan, preview renders, and critique
// rounds.
func (s *Store) SaveRunWork(id string, todos []model.Todo, renders []model.Render, critiques []model.Critique) error {
	cols := make([]any, 0, 4)
	for _, v := range []any{todos, renders, critiques} {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		cols = append(cols, string(b))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE runs SET todos=?, renders=?, critiques=? WHERE id=?`, append(cols, id)...)
	return err
}

const runColumns = `id,project_id,COALESCE(conversation_id,''),status,COALESCE(prompt,''),
		COALESCE(error,''),started_at,ended_at,COALESCE(todos,''),COALESCE(renders,''),COALESCE(critiques,'')`

// runHeaderColumns matches runColumns' shape without the prompt and work JSON.
const runHeaderColumns = `id,project_id,COALESCE(conversation_id,''),status,'',
		COALESCE(error,''),started_at,ended_at,'','',''`

func scanRun(scan func(...any) error) (model.Run, error) {
	var r model.Run
	var started int64
	var ended sql.NullInt64
	var todos, renders, critiques string
	if err := scan(&r.ID, &r.ProjectID, &r.ConversationID, &r.Status, &r.Prompt, &r.Error, &started, &ended, &todos, &renders, &critiques); err != nil {
		return r, err
	}
	r.StartedAt = time.UnixMilli(started)
	if ended.Valid {
		t := time.UnixMilli(ended.Int64)
		r.EndedAt = &t
	}
	// Rows written before these columns existed have no work recorded.
	if todos != "" {
		_ = json.Unmarshal([]byte(todos), &r.Todos)
	}
	if renders != "" {
		_ = json.Unmarshal([]byte(renders), &r.Renders)
	}
	if critiques != "" {
		_ = json.Unmarshal([]byte(critiques), &r.Critiques)
	}
	return r, nil
}

// GetRun loads one run.
func (s *Store) GetRun(id string) (model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := scanRun(s.db.QueryRow(`SELECT `+runColumns+` FROM runs WHERE id=?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// RunStatus reads only a run's status, for loops that poll for completion.
func (s *Store) RunStatus(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var status string
	err := s.db.QueryRow(`SELECT status FROM runs WHERE id=?`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return status, err
}

// FailStaleRuns marks runs that were still marked running as failed. A daemon
// restart terminates any in-flight agent process, so those runs can never
// complete on their own.
func (s *Store) FailStaleRuns() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(
		`UPDATE runs SET status=?, error=?, ended_at=? WHERE status=?`,
		model.RunFailed, "daemon restarted during the run", time.Now().UnixMilli(), model.RunRunning)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ListRuns returns a project's runs, newest first.
func (s *Store) ListRuns(projectID string) ([]model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT `+runColumns+` FROM runs WHERE project_id=? ORDER BY started_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Run
	for rows.Next() {
		r, err := scanRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ------------------------------------------------------------------ utils

// Slugify converts a display name into a filesystem- and URL-safe slug.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '\\':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
