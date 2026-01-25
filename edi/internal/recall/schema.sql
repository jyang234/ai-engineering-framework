-- RECALL v0 SQLite Schema
-- Uses FTS5 for full-text search

-- Knowledge items table
CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,  -- pattern, failure, decision, context
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,           -- JSON array
    scope TEXT NOT NULL, -- global, project
    project_path TEXT,   -- NULL for global
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    usefulness_score REAL DEFAULT 0.0,
    use_count INTEGER DEFAULT 0
);

-- Full-text search index using FTS5
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
    title,
    content,
    tags,
    content=items,
    content_rowid=rowid
);

-- Triggers to keep FTS in sync with items table
CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
    INSERT INTO items_fts(rowid, title, content, tags)
    VALUES (NEW.rowid, NEW.title, NEW.content, NEW.tags);
END;

CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, title, content, tags)
    VALUES('delete', OLD.rowid, OLD.title, OLD.content, OLD.tags);
END;

CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, title, content, tags)
    VALUES('delete', OLD.rowid, OLD.title, OLD.content, OLD.tags);
    INSERT INTO items_fts(rowid, title, content, tags)
    VALUES (NEW.rowid, NEW.title, NEW.content, NEW.tags);
END;

-- Feedback table for tracking item usefulness
CREATE TABLE IF NOT EXISTS feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    useful BOOLEAN NOT NULL,
    context TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (item_id) REFERENCES items(id)
);

-- Flight recorder table for session events
CREATE TABLE IF NOT EXISTS flight_recorder (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    type TEXT NOT NULL,  -- decision, error, milestone, observation, task_annotation, task_complete
    content TEXT NOT NULL,
    rationale TEXT,
    metadata TEXT        -- JSON
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_items_type ON items(type);
CREATE INDEX IF NOT EXISTS idx_items_scope ON items(scope);
CREATE INDEX IF NOT EXISTS idx_items_project ON items(project_path);
CREATE INDEX IF NOT EXISTS idx_feedback_item ON feedback(item_id);
CREATE INDEX IF NOT EXISTS idx_flight_session ON flight_recorder(session_id);
CREATE INDEX IF NOT EXISTS idx_flight_type ON flight_recorder(type);
