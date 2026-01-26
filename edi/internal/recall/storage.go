package recall

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// Storage handles SQLite operations for RECALL
type Storage struct {
	db *sql.DB
}

// Item represents a knowledge item
type Item struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	Tags            []string  `json:"tags"`
	Scope           string    `json:"scope"`
	ProjectPath     string    `json:"project_path,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	UsefulnessScore float64   `json:"usefulness_score"`
	UseCount        int       `json:"use_count"`
}

// FlightRecorderEntry represents a flight recorder log entry
type FlightRecorderEntry struct {
	ID        int64                  `json:"id"`
	SessionID string                 `json:"session_id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Rationale string                 `json:"rationale,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewStorage creates a new Storage instance
func NewStorage(dbPath string) (*Storage, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_fk=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Storage{db: db}, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// Search performs FTS search on knowledge items
func (s *Storage) Search(query string, types []string, scope string, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build FTS query
	ftsQuery := `
		SELECT i.id, i.type, i.title, i.content, i.tags, i.scope,
		       i.project_path, i.created_at, i.updated_at,
		       i.usefulness_score, i.use_count
		FROM items i
		JOIN items_fts fts ON i.rowid = fts.rowid
		WHERE items_fts MATCH ?
	`

	args := []interface{}{query}

	if len(types) > 0 {
		placeholders := make([]string, len(types))
		for i, t := range types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		ftsQuery += fmt.Sprintf(" AND i.type IN (%s)", strings.Join(placeholders, ","))
	}

	if scope != "" && scope != "all" {
		ftsQuery += " AND i.scope = ?"
		args = append(args, scope)
	}

	ftsQuery += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(ftsQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		var tagsJSON sql.NullString
		var projectPath sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(
			&item.ID, &item.Type, &item.Title, &item.Content,
			&tagsJSON, &item.Scope, &projectPath,
			&createdAt, &updatedAt,
			&item.UsefulnessScore, &item.UseCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		if tagsJSON.Valid {
			if err := json.Unmarshal([]byte(tagsJSON.String), &item.Tags); err != nil {
				log.Printf("warning: failed to parse tags for item %s: %v", item.ID, err)
			}
		}
		if projectPath.Valid {
			item.ProjectPath = projectPath.String
		}

		if t, err := time.Parse(time.RFC3339, createdAt); err != nil {
			log.Printf("warning: failed to parse created_at for item %s: %v", item.ID, err)
		} else {
			item.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, updatedAt); err != nil {
			log.Printf("warning: failed to parse updated_at for item %s: %v", item.ID, err)
		} else {
			item.UpdatedAt = t
		}

		items = append(items, item)
	}

	return items, nil
}

// Add adds a new item to the knowledge base
func (s *Storage) Add(item *Item) error {
	tagsJSON, err := json.Marshal(item.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO items (id, type, title, content, tags, scope, project_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		item.ID, item.Type, item.Title, item.Content,
		string(tagsJSON), item.Scope, item.ProjectPath,
		item.CreatedAt.Format(time.RFC3339),
		item.UpdatedAt.Format(time.RFC3339),
	)

	return err
}

// Get retrieves an item by ID
func (s *Storage) Get(id string) (*Item, error) {
	row := s.db.QueryRow(`
		SELECT id, type, title, content, tags, scope, project_path,
		       created_at, updated_at, usefulness_score, use_count
		FROM items WHERE id = ?
	`, id)

	var item Item
	var tagsJSON sql.NullString
	var projectPath sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&item.ID, &item.Type, &item.Title, &item.Content,
		&tagsJSON, &item.Scope, &projectPath,
		&createdAt, &updatedAt,
		&item.UsefulnessScore, &item.UseCount,
	)
	if err != nil {
		return nil, err
	}

	if tagsJSON.Valid {
		if err := json.Unmarshal([]byte(tagsJSON.String), &item.Tags); err != nil {
			log.Printf("warning: failed to parse tags for item %s: %v", item.ID, err)
		}
	}
	if projectPath.Valid {
		item.ProjectPath = projectPath.String
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err != nil {
		log.Printf("warning: failed to parse created_at for item %s: %v", item.ID, err)
	} else {
		item.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		log.Printf("warning: failed to parse updated_at for item %s: %v", item.ID, err)
	} else {
		item.UpdatedAt = t
	}

	return &item, nil
}

// RecordFeedback records usefulness feedback for an item
func (s *Storage) RecordFeedback(itemID, sessionID string, useful bool, context string) error {
	_, err := s.db.Exec(`
		INSERT INTO feedback (item_id, session_id, useful, context, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, itemID, sessionID, useful, context, time.Now().Format(time.RFC3339))

	if err != nil {
		return err
	}

	// Update usefulness score
	if useful {
		_, err = s.db.Exec(`
			UPDATE items
			SET usefulness_score = usefulness_score + 1.0,
			    use_count = use_count + 1
			WHERE id = ?
		`, itemID)
	}

	return err
}

// LogFlightRecorder logs an entry to the flight recorder
func (s *Storage) LogFlightRecorder(entry *FlightRecorderEntry) error {
	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO flight_recorder (session_id, timestamp, type, content, rationale, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		entry.SessionID,
		entry.Timestamp.Format(time.RFC3339),
		entry.Type,
		entry.Content,
		entry.Rationale,
		string(metadataJSON),
	)

	return err
}

// GetFlightRecorderEntries retrieves flight recorder entries for a session
func (s *Storage) GetFlightRecorderEntries(sessionID string) ([]FlightRecorderEntry, error) {
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

	var entries []FlightRecorderEntry
	for rows.Next() {
		var entry FlightRecorderEntry
		var timestamp string
		var rationale sql.NullString
		var metadataJSON sql.NullString

		err := rows.Scan(
			&entry.ID, &entry.SessionID, &timestamp,
			&entry.Type, &entry.Content, &rationale, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if t, err := time.Parse(time.RFC3339, timestamp); err != nil {
			log.Printf("warning: failed to parse timestamp for flight recorder entry %d: %v", entry.ID, err)
		} else {
			entry.Timestamp = t
		}
		if rationale.Valid {
			entry.Rationale = rationale.String
		}
		if metadataJSON.Valid {
			if err := json.Unmarshal([]byte(metadataJSON.String), &entry.Metadata); err != nil {
				log.Printf("warning: failed to parse metadata for flight recorder entry %d: %v", entry.ID, err)
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
