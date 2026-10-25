package history

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const timeFmt = time.RFC3339Nano

// NewConversation creates a conversation and returns it.
func (s *Store) NewConversation(title, model, projectID string) (Conversation, error) {
	c := Conversation{
		ID: newID(), Title: title, Model: model, ProjectID: projectID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	_, err := s.db.Exec(
		`INSERT INTO conversations (id, title, model, folder, pinned, project_id, created_at, updated_at)
		 VALUES (?, ?, ?, '', 0, ?, ?, ?)`,
		c.ID, c.Title, c.Model, c.ProjectID, c.CreatedAt.Format(timeFmt), c.UpdatedAt.Format(timeFmt),
	)
	return c, err
}

// ListConversations returns conversations, most recently updated first.
func (s *Store) ListConversations() ([]Conversation, error) {
	rows, err := s.db.Query(`SELECT id, title, model, folder, pinned, project_id, created_at, updated_at
		FROM conversations ORDER BY pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		var pinned int
		var created, updated string
		if err := rows.Scan(&c.ID, &c.Title, &c.Model, &c.Folder, &pinned, &c.ProjectID, &created, &updated); err != nil {
			return nil, err
		}
		c.Pinned = pinned != 0
		c.CreatedAt, _ = time.Parse(timeFmt, created)
		c.UpdatedAt, _ = time.Parse(timeFmt, updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Pin sets a conversation's pinned flag.
func (s *Store) Pin(conversationID string, pinned bool) error {
	_, err := s.db.Exec(`UPDATE conversations SET pinned = ? WHERE id = ?`, boolToInt(pinned), conversationID)
	return err
}

// SetFolder moves a conversation into a folder ("" clears it).
func (s *Store) SetFolder(conversationID, folder string) error {
	_, err := s.db.Exec(`UPDATE conversations SET folder = ? WHERE id = ?`, folder, conversationID)
	return err
}

// Rename sets a conversation's title.
func (s *Store) Rename(conversationID, title string) error {
	_, err := s.db.Exec(`UPDATE conversations SET title = ? WHERE id = ?`, title, conversationID)
	return err
}

// AddMessage appends a message to a branch of a conversation and bumps
// the conversation's updated_at.
func (s *Store) AddMessage(conversationID, parentID, branchID, role, content string) (Message, error) {
	if branchID == "" {
		branchID = "main"
	}
	m := Message{
		ID: newID(), ConversationID: conversationID, ParentID: parentID,
		BranchID: branchID, Role: role, Content: content, CreatedAt: time.Now(),
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO messages (id, conversation_id, parent_id, branch_id, role, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.ParentID, m.BranchID, m.Role, m.Content, m.CreatedAt.Format(timeFmt),
	); err != nil {
		return Message{}, err
	}
	if _, err := tx.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, m.CreatedAt.Format(timeFmt), conversationID); err != nil {
		return Message{}, err
	}
	return m, tx.Commit()
}

// Messages returns every message on a branch, oldest first.
func (s *Store) Messages(conversationID, branchID string) ([]Message, error) {
	if branchID == "" {
		branchID = "main"
	}
	rows, err := s.db.Query(
		`SELECT id, conversation_id, parent_id, branch_id, role, content, created_at
		 FROM messages WHERE conversation_id = ? AND branch_id = ? ORDER BY created_at ASC`,
		conversationID, branchID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var created string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.ParentID, &m.BranchID, &m.Role, &m.Content, &created); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(timeFmt, created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// Branches lists every distinct branch id in a conversation.
func (s *Store) Branches(conversationID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT branch_id FROM messages WHERE conversation_id = ?`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Fork creates a new branch from fromMessageID (inclusive of its
// ancestors, exclusive of anything after it), used when the user edits a
// past message: the edited message starts a new branch.
func (s *Store) Fork(conversationID, fromMessageID, newBranchID, editedContent string) error {
	msgs, err := s.allAncestors(conversationID, fromMessageID)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var parent string
	for i, m := range msgs {
		content := m.Content
		if i == len(msgs)-1 {
			content = editedContent
		}
		newMsgID := newID()
		if _, err := tx.Exec(
			`INSERT INTO messages (id, conversation_id, parent_id, branch_id, role, content, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newMsgID, conversationID, parent, newBranchID, m.Role, content, time.Now().Format(timeFmt),
		); err != nil {
			return err
		}
		parent = newMsgID
	}
	return tx.Commit()
}

func (s *Store) allAncestors(conversationID, messageID string) ([]Message, error) {
	// Simple approach: load the whole branch the message belongs to, up
	// to and including that message, in chronological order.
	var branchID string
	if err := s.db.QueryRow(`SELECT branch_id FROM messages WHERE id = ?`, messageID).Scan(&branchID); err != nil {
		return nil, fmt.Errorf("find message %s: %w", messageID, err)
	}
	all, err := s.Messages(conversationID, branchID)
	if err != nil {
		return nil, err
	}
	var out []Message
	for _, m := range all {
		out = append(out, m)
		if m.ID == messageID {
			break
		}
	}
	return out, nil
}

// Search does a full-text search across all messages, returning matching
// messages newest first.
func (s *Store) Search(query string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT m.id, m.conversation_id, m.parent_id, m.branch_id, m.role, m.content, m.created_at
		 FROM messages m JOIN messages_fts f ON m.rowid = f.rowid
		 WHERE messages_fts MATCH ? ORDER BY m.created_at DESC LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var created string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.ParentID, &m.BranchID, &m.Role, &m.Content, &created); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(timeFmt, created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ExportMarkdown renders a branch as a Markdown transcript.
func (s *Store) ExportMarkdown(conversationID, branchID string) (string, error) {
	msgs, err := s.Messages(conversationID, branchID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "### %s (%s)\n\n%s\n\n", strings.ToUpper(m.Role[:1])+m.Role[1:], m.CreatedAt.Format("2006-01-02 15:04"), m.Content)
	}
	return b.String(), nil
}

// ExportJSON renders a branch as JSON.
func (s *Store) ExportJSON(conversationID, branchID string) ([]byte, error) {
	msgs, err := s.Messages(conversationID, branchID)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(msgs, "", "  ")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
