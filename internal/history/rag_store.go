package history

import (
	"encoding/binary"
	"math"
	"time"
)

// DocChunk is one indexed, embedded chunk of a project file, used for
// local RAG (item 16). The embedding is stored as a flat float32 blob;
// there's no need for a sqlite-vec extension at this scale — a linear
// cosine-similarity scan in Go over a few thousand chunks is well under
// a millisecond and keeps the pure-Go SQLite driver (no C extensions).
type DocChunk struct {
	ID        string
	ProjectID string
	FileName  string
	ChunkIdx  int
	Text      string
	Embedding []float32
	CreatedAt time.Time
}

func (s *Store) migrateRAG() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS doc_chunks (
	id         TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	file_name  TEXT NOT NULL,
	chunk_idx  INTEGER NOT NULL,
	text       TEXT NOT NULL,
	embedding  BLOB NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_doc_chunks_project ON doc_chunks(project_id);
`)
	return err
}

// AddChunk stores one embedded chunk.
func (s *Store) AddChunk(c DocChunk) error {
	if c.ID == "" {
		c.ID = newID()
	}
	c.CreatedAt = time.Now()
	_, err := s.db.Exec(
		`INSERT INTO doc_chunks (id, project_id, file_name, chunk_idx, text, embedding, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.FileName, c.ChunkIdx, c.Text, encodeEmbedding(c.Embedding), c.CreatedAt.Format(timeFmt),
	)
	return err
}

// Chunks returns every indexed chunk for a project, for similarity
// search (see internal/rag.Search, which calls this and ranks results).
func (s *Store) Chunks(projectID string) ([]DocChunk, error) {
	rows, err := s.db.Query(`SELECT id, project_id, file_name, chunk_idx, text, embedding, created_at FROM doc_chunks WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DocChunk
	for rows.Next() {
		var c DocChunk
		var blob []byte
		var created string
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.FileName, &c.ChunkIdx, &c.Text, &blob, &created); err != nil {
			return nil, err
		}
		c.Embedding = decodeEmbedding(blob)
		c.CreatedAt, _ = time.Parse(timeFmt, created)
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteChunksForFile removes all chunks previously indexed for a given
// file name within a project, so re-indexing doesn't duplicate.
func (s *Store) DeleteChunksForFile(projectID, fileName string) error {
	_, err := s.db.Exec(`DELETE FROM doc_chunks WHERE project_id = ? AND file_name = ?`, projectID, fileName)
	return err
}

func encodeEmbedding(v []float32) []byte {
	buf := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeEmbedding(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
