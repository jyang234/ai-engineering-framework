package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

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

	db, err := sql.Open("sqlite3", dbPath)
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
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *MetadataStore) Close() error {
	return s.db.Close()
}

// SaveItem saves an item to the metadata store
func (s *MetadataStore) SaveItem(item *ItemRecord) error {
	tagsJSON, _ := json.Marshal(item.Tags)
	metaJSON, _ := json.Marshal(item.Metadata)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO items (id, type, title, content, tags, scope, source, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

	json.Unmarshal([]byte(tagsJSON), &item.Tags)
	json.Unmarshal([]byte(metaJSON), &item.Metadata)

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
	metaJSON, _ := json.Marshal(entry.Metadata)

	_, err := s.db.Exec(`
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

		json.Unmarshal([]byte(metaJSON), &entry.Metadata)
		entries = append(entries, &entry)
	}

	return entries, nil
}
