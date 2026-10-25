package history

import "time"

// Project is a workspace with its own system prompt, files, and memory
// (FEATURES.md item 15). Conversations can optionally belong to one via
// Conversation.ProjectID.
type Project struct {
	ID           string
	Name         string
	SystemPrompt string
	FilesDir     string // directory holding project-attached files, indexed by RAG
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s *Store) migrateProjects() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS projects (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL,
	system_prompt TEXT NOT NULL DEFAULT '',
	files_dir     TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS user_memory (
	id         TEXT PRIMARY KEY,
	project_id TEXT NOT NULL DEFAULT '',
	fact       TEXT NOT NULL,
	created_at TEXT NOT NULL
);
`)
	return err
}

// NewProject creates a project.
func (s *Store) NewProject(name, systemPrompt, filesDir string) (Project, error) {
	p := Project{ID: newID(), Name: name, SystemPrompt: systemPrompt, FilesDir: filesDir, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_, err := s.db.Exec(`INSERT INTO projects (id, name, system_prompt, files_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.SystemPrompt, p.FilesDir, p.CreatedAt.Format(timeFmt), p.UpdatedAt.Format(timeFmt))
	return p, err
}

// ListProjects returns all projects.
func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id, name, system_prompt, files_dir, created_at, updated_at FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var created, updated string
		if err := rows.Scan(&p.ID, &p.Name, &p.SystemPrompt, &p.FilesDir, &created, &updated); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(timeFmt, created)
		p.UpdatedAt, _ = time.Parse(timeFmt, updated)
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProject looks up a project by ID.
func (s *Store) GetProject(id string) (Project, bool, error) {
	var p Project
	var created, updated string
	err := s.db.QueryRow(`SELECT id, name, system_prompt, files_dir, created_at, updated_at FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.SystemPrompt, &p.FilesDir, &created, &updated)
	if err != nil {
		return Project{}, false, nilIfNoRows(err)
	}
	p.CreatedAt, _ = time.Parse(timeFmt, created)
	p.UpdatedAt, _ = time.Parse(timeFmt, updated)
	return p, true, nil
}

// UpdateProjectPrompt updates a project's system prompt.
func (s *Store) UpdateProjectPrompt(id, systemPrompt string) error {
	_, err := s.db.Exec(`UPDATE projects SET system_prompt = ?, updated_at = ? WHERE id = ?`, systemPrompt, time.Now().Format(timeFmt), id)
	return err
}

// MemoryFact is one persistent, user-visible memory item (item 18 — never
// hidden from the user).
type MemoryFact struct {
	ID        string
	ProjectID string // "" for global memory
	Fact      string
	CreatedAt time.Time
}

// AddMemory records a new memory fact.
func (s *Store) AddMemory(projectID, fact string) (MemoryFact, error) {
	m := MemoryFact{ID: newID(), ProjectID: projectID, Fact: fact, CreatedAt: time.Now()}
	_, err := s.db.Exec(`INSERT INTO user_memory (id, project_id, fact, created_at) VALUES (?, ?, ?, ?)`,
		m.ID, m.ProjectID, m.Fact, m.CreatedAt.Format(timeFmt))
	return m, err
}

// ListMemory returns memory facts for a project (or global memory when
// projectID is "").
func (s *Store) ListMemory(projectID string) ([]MemoryFact, error) {
	rows, err := s.db.Query(`SELECT id, project_id, fact, created_at FROM user_memory WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryFact
	for rows.Next() {
		var m MemoryFact
		var created string
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Fact, &created); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(timeFmt, created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMemory removes a memory fact — the user must always be able to
// forget something Runyx remembered about them.
func (s *Store) DeleteMemory(id string) error {
	_, err := s.db.Exec(`DELETE FROM user_memory WHERE id = ?`, id)
	return err
}

func nilIfNoRows(err error) error {
	if err == nil {
		return nil
	}
	if err.Error() == "sql: no rows in result set" {
		return nil
	}
	return err
}
