package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// GenerateID creates a new UUID for an item.
func GenerateID() string {
	return uuid.New().String()
}

// MetadataStore handles SQLite metadata storage
type MetadataStore struct {
	db *sql.DB
}

// ItemRecord represents an item in the metadata store
type ItemRecord struct {
	ID        string
	Type      string
	Title     string
	Content   string
	Tags      []string
	Scope     string
	Source    string
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FeedbackRecord represents feedback on an item
type FeedbackRecord struct {
	ID        string
	ItemID    string
	SessionID string
	Useful    bool
	Context   string
	Timestamp time.Time
}

// FlightRecorderRecord represents a flight recorder entry
type FlightRecorderRecord struct {
	ID        string
	SessionID string
	Timestamp time.Time
	Type      string
	Content   string
	Rationale string
	Metadata  map[string]any
}

// NewMetadataStore creates a new metadata store
func NewMetadataStore(dbPath string) (*MetadataStore, error) {
	// Expand ~ in path
	if strings.HasPrefix(dbPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dbPath = filepath.Join(home, dbPath[1:])
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, err
	}

	store := &MetadataStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// migrate creates the necessary tables
func (s *MetadataStore) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			scope TEXT NOT NULL DEFAULT 'project',
			source TEXT,
			metadata TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS feedback (
			id TEXT PRIMARY KEY,
			item_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			useful INTEGER NOT NULL,
			context TEXT,
			timestamp DATETIME NOT NULL,
			FOREIGN KEY (item_id) REFERENCES items(id)
		);

		CREATE TABLE IF NOT EXISTS flight_recorder (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			rationale TEXT,
			metadata TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_items_type ON items(type);
		CREATE INDEX IF NOT EXISTS idx_items_scope ON items(scope);
		CREATE INDEX IF NOT EXISTS idx_feedback_item ON feedback(item_id);
		CREATE INDEX IF NOT EXISTS idx_flight_session ON flight_recorder(session_id);
		CREATE INDEX IF NOT EXISTS idx_flight_type ON flight_recorder(type);

		CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
			title, content, tags,
			content=items, content_rowid=rowid,
			tokenize='porter unicode61'
		);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Create triggers to keep FTS in sync with items table
	triggers := `
		CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
			INSERT INTO items_fts(rowid, title, content, tags)
			VALUES (new.rowid, new.title, new.content, new.tags);
		END;

		CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, content, tags)
			VALUES ('delete', old.rowid, old.title, old.content, old.tags);
		END;

		CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, content, tags)
			VALUES ('delete', old.rowid, old.title, old.content, old.tags);
			INSERT INTO items_fts(rowid, title, content, tags)
			VALUES (new.rowid, new.title, new.content, new.tags);
		END;
	`
	if _, err = s.db.Exec(triggers); err != nil {
		return err
	}

	// Rebuild FTS index only if it's empty but items table has data.
	// This handles first-time setup and recovery; triggers keep it in sync otherwise.
	var ftsCount, itemCount int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM items_fts").Scan(&ftsCount); err != nil {
		return fmt.Errorf("count items_fts: %w", err)
	}
	if err = s.db.QueryRow("SELECT COUNT(*) FROM items").Scan(&itemCount); err != nil {
		return fmt.Errorf("count items: %w", err)
	}
	if ftsCount == 0 && itemCount > 0 {
		_, err = s.db.Exec("INSERT INTO items_fts(items_fts) VALUES('rebuild')")
		return err
	}
	return nil
}

// DB returns the underlying database connection.
// Used to share the SQLite connection with VecStore.
func (s *MetadataStore) DB() *sql.DB {
	return s.db
}

// Close closes the database connection
func (s *MetadataStore) Close() error {
	return s.db.Close()
}

// SaveItem saves an item to the metadata store
func (s *MetadataStore) SaveItem(item *ItemRecord) error {
	tagsJSON, err := json.Marshal(item.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	metaJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO items (id, type, title, content, tags, scope, source, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type=excluded.type, title=excluded.title, content=excluded.content,
			tags=excluded.tags, scope=excluded.scope, source=excluded.source,
			metadata=excluded.metadata, updated_at=excluded.updated_at
	`, item.ID, item.Type, item.Title, item.Content, string(tagsJSON), item.Scope, item.Source, string(metaJSON), item.CreatedAt, item.UpdatedAt)

	return err
}

// GetItem retrieves an item by ID
func (s *MetadataStore) GetItem(id string) (*ItemRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, type, title, content, tags, scope, source, metadata, created_at, updated_at
		FROM items WHERE id = ?
	`, id)

	var item ItemRecord
	var tagsJSON, metaJSON string

	err := row.Scan(&item.ID, &item.Type, &item.Title, &item.Content, &tagsJSON, &item.Scope, &item.Source, &metaJSON, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("item not found: %s", id)
		}
		return nil, err
	}

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags for %s: %w", id, err)
		}
	}
	if metaJSON != "" {
		if err := json.Unmarshal([]byte(metaJSON), &item.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata for %s: %w", id, err)
		}
	}

	return &item, nil
}

// RecordFeedback records feedback on an item
func (s *MetadataStore) RecordFeedback(feedback *FeedbackRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO feedback (id, item_id, session_id, useful, context, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, feedback.ID, feedback.ItemID, feedback.SessionID, feedback.Useful, feedback.Context, feedback.Timestamp)

	return err
}

// LogFlightRecorder logs an entry to the flight recorder
func (s *MetadataStore) LogFlightRecorder(entry *FlightRecorderRecord) error {
	metaJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshal flight recorder metadata: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO flight_recorder (id, session_id, timestamp, type, content, rationale, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, entry.ID, entry.SessionID, entry.Timestamp, entry.Type, entry.Content, entry.Rationale, string(metaJSON))

	return err
}

// GetFlightRecorderEntries retrieves flight recorder entries for a session
func (s *MetadataStore) GetFlightRecorderEntries(sessionID string) ([]*FlightRecorderRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, timestamp, type, content, rationale, metadata
		FROM flight_recorder
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*FlightRecorderRecord
	for rows.Next() {
		var entry FlightRecorderRecord
		var metaJSON string

		err := rows.Scan(&entry.ID, &entry.SessionID, &entry.Timestamp, &entry.Type, &entry.Content, &entry.Rationale, &metaJSON)
		if err != nil {
			return nil, err
		}

		if metaJSON != "" {
			if err := json.Unmarshal([]byte(metaJSON), &entry.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal flight recorder metadata: %w", err)
			}
		}
		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}

// ListItems retrieves items with optional filters and pagination
func (s *MetadataStore) ListItems(itemType, scope string, limit, offset int) ([]*ItemRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	query := "SELECT id, type, title, content, tags, scope, source, metadata, created_at, updated_at FROM items WHERE 1=1"
	args := []any{}

	if itemType != "" {
		query += " AND type = ?"
		args = append(args, itemType)
	}

	if scope != "" {
		query += " AND scope = ?"
		args = append(args, scope)
	}

	query += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*ItemRecord
	for rows.Next() {
		var item ItemRecord
		var tagsJSON, metaJSON string

		err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Content, &tagsJSON, &item.Scope, &item.Source, &metaJSON, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if tagsJSON != "" {
			if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
				return nil, fmt.Errorf("unmarshal tags: %w", err)
			}
		}
		if metaJSON != "" {
			if err := json.Unmarshal([]byte(metaJSON), &item.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}
		items = append(items, &item)
	}

	return items, rows.Err()
}

// KeywordSearch performs FTS5 full-text search on items.
// Returns results ranked by BM25 relevance score.
func (s *MetadataStore) KeywordSearch(query string, limit int) ([]KeywordResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	// Sanitize query for FTS5: escape double quotes and wrap in quotes
	// to prevent FTS5 syntax errors from special characters.
	// NOTE: This disables FTS5 advanced syntax (AND, OR, NEAR, *) by design —
	// the query is treated as a literal phrase match for safety.
	sanitized := strings.ReplaceAll(query, `"`, `""`)
	sanitized = `"` + sanitized + `"`

	rows, err := s.db.Query(`
		SELECT i.id, i.type, i.title, i.content, i.tags, i.scope,
		       -rank AS score
		FROM items_fts f
		JOIN items i ON i.rowid = f.rowid
		WHERE items_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, sanitized, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []KeywordResult
	for rows.Next() {
		var r KeywordResult
		var tagsJSON string
		err := rows.Scan(&r.ID, &r.Type, &r.Title, &r.Content, &tagsJSON, &r.Scope, &r.Score)
		if err != nil {
			return nil, err
		}
		if tagsJSON != "" {
			if err := json.Unmarshal([]byte(tagsJSON), &r.Tags); err != nil {
				return nil, fmt.Errorf("unmarshal tags: %w", err)
			}
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// KeywordResult represents a keyword search result with BM25 score.
type KeywordResult struct {
	ID      string
	Type    string
	Title   string
	Content string
	Tags    []string
	Scope   string
	Score   float64
}

// DeleteItem removes an item from the metadata store
func (s *MetadataStore) DeleteItem(id string) error {
	_, err := s.db.Exec("DELETE FROM items WHERE id = ?", id)
	return err
}

// CountItems returns count of items, optionally filtered by type
func (s *MetadataStore) CountItems(itemType string) (int, error) {
	var count int
	var err error
	if itemType == "" {
		err = s.db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	} else {
		err = s.db.QueryRow("SELECT COUNT(*) FROM items WHERE type = ?", itemType).Scan(&count)
	}
	return count, err
}

// CountItemsByType returns counts grouped by type
func (s *MetadataStore) CountItemsByType() (map[string]int, error) {
	rows, err := s.db.Query("SELECT type, COUNT(*) FROM items GROUP BY type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var itemType string
		var count int
		if err := rows.Scan(&itemType, &count); err != nil {
			return nil, err
		}
		counts[itemType] = count
	}
	return counts, rows.Err()
}
