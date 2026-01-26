package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var manifestLock sync.Mutex

// ManifestPath returns the path to the active tasks file for a project
// Note: Renamed from manifest.yaml to active.yaml to clarify that only active tasks are stored
func ManifestPath(projectPath string) string {
	return filepath.Join(projectPath, ".edi", "tasks", "active.yaml")
}

// legacyManifestPath returns the old manifest.yaml path for migration
func legacyManifestPath(projectPath string) string {
	return filepath.Join(projectPath, ".edi", "tasks", "manifest.yaml")
}

// LoadManifest loads the task manifest from disk
// Returns an empty manifest if the file doesn't exist
// Handles migration from legacy manifest.yaml to active.yaml
func LoadManifest(projectPath string) (*Manifest, error) {
	path := ManifestPath(projectPath)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check for legacy manifest.yaml and migrate
			legacyPath := legacyManifestPath(projectPath)
			legacyData, legacyErr := os.ReadFile(legacyPath)
			if legacyErr == nil {
				var manifest Manifest
				if err := yaml.Unmarshal(legacyData, &manifest); err != nil {
					return nil, fmt.Errorf("failed to parse legacy manifest: %w", err)
				}
				// Remove completed tasks during migration
				manifest.RemoveCompletedTasks()
				// Save to new location and remove legacy file
				if saveErr := SaveManifest(projectPath, &manifest); saveErr == nil {
					os.Remove(legacyPath)
				}
				return &manifest, nil
			}
			return NewManifest(), nil
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// SaveManifest saves the task manifest to disk
func SaveManifest(projectPath string, manifest *Manifest) error {
	manifestLock.Lock()
	defer manifestLock.Unlock()

	path := ManifestPath(projectPath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	manifest.LastSync = time.Now()

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename manifest: %w", err)
	}

	return nil
}

// SessionInfo holds information about a Claude Code task session
type SessionInfo struct {
	ID       string
	Path     string
	ModTime  time.Time
	NumTasks int
}

// ScanClaudeSessions finds all Claude Code task sessions
func ScanClaudeSessions() ([]SessionInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	tasksDir := filepath.Join(home, ".claude", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionPath := filepath.Join(tasksDir, entry.Name())
		taskFiles, _ := filepath.Glob(filepath.Join(sessionPath, "*.json"))

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sessions = append(sessions, SessionInfo{
			ID:       entry.Name(),
			Path:     sessionPath,
			ModTime:  info.ModTime(),
			NumTasks: len(taskFiles),
		})
	}

	// Sort by mod time, newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	return sessions, nil
}

// ScanClaudeTasks scans Claude Code task directories for tasks newer than lastSync
func ScanClaudeTasks(lastSync time.Time) (map[string][]ClaudeTask, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	tasksDir := filepath.Join(home, ".claude", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]ClaudeTask), nil
		}
		return nil, err
	}

	result := make(map[string][]ClaudeTask)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		sessionPath := filepath.Join(tasksDir, sessionID)

		// Check session directory mod time
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Skip sessions that haven't been modified since last sync
		if !lastSync.IsZero() && info.ModTime().Before(lastSync) {
			continue
		}

		tasks, err := loadClaudeTasksFromSession(sessionPath)
		if err != nil {
			continue
		}

		if len(tasks) > 0 {
			result[sessionID] = tasks
		}
	}

	return result, nil
}

// loadClaudeTasksFromSession loads all tasks from a Claude Code session directory
func loadClaudeTasksFromSession(sessionPath string) ([]ClaudeTask, error) {
	entries, err := os.ReadDir(sessionPath)
	if err != nil {
		return nil, err
	}

	var tasks []ClaudeTask
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		taskPath := filepath.Join(sessionPath, entry.Name())
		data, err := os.ReadFile(taskPath)
		if err != nil {
			continue
		}

		var task ClaudeTask
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ReconcileTasks merges Claude tasks into the manifest
// Uses timestamp-based reconciliation: newer updates win
// After reconciliation, completed tasks are removed (relay-only approach)
func ReconcileTasks(manifest *Manifest, sessionTasks map[string][]ClaudeTask) int {
	for sessionID, tasks := range sessionTasks {
		for _, ct := range tasks {
			existing := manifest.FindTask(ct.ID)

			// Get the file mod time for this task
			home, _ := os.UserHomeDir()
			taskPath := filepath.Join(home, ".claude", "tasks", sessionID, ct.ID+".json")
			info, _ := os.Stat(taskPath)

			var modTime time.Time
			if info != nil {
				modTime = info.ModTime()
			} else {
				modTime = time.Now()
			}

			if existing == nil {
				// New task - add to manifest only if not completed
				if ct.Status != "completed" && ct.Status != "done" {
					manifest.UpsertTask(ct.ToTask(modTime))
				}
			} else if modTime.After(existing.UpdatedAt) {
				// Claude's version is newer - update manifest
				task := ct.ToTask(modTime)
				task.CreatedAt = existing.CreatedAt // Preserve original creation time
				manifest.UpsertTask(task)
			}
			// Otherwise manifest version is newer or same - keep it
		}
	}

	// Remove completed tasks after reconciliation (relay-only approach)
	return manifest.RemoveCompletedTasks()
}

// HydrateClaudeStore creates task files in Claude's task directory for a new session
func HydrateClaudeStore(sessionID string, tasks []Task) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sessionPath := filepath.Join(home, ".claude", "tasks", sessionID)
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	for _, task := range tasks {
		ct := task.ToClaudeTask()
		data, err := json.MarshalIndent(ct, "", "  ")
		if err != nil {
			continue
		}

		taskPath := filepath.Join(sessionPath, task.ID+".json")
		if err := os.WriteFile(taskPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write task %s: %w", task.ID, err)
		}
	}

	return nil
}

// SyncOnLaunch performs full task synchronization when EDI launches
// Returns the new session ID to use
func SyncOnLaunch(projectPath string) (string, error) {
	// Check if this is an EDI project
	ediDir := filepath.Join(projectPath, ".edi")
	if _, err := os.Stat(ediDir); os.IsNotExist(err) {
		// Not an EDI project - no sync needed
		return "", nil
	}

	// Load manifest
	manifest, err := LoadManifest(projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to load manifest: %w", err)
	}

	// Scan Claude tasks for changes since last sync
	sessionTasks, err := ScanClaudeTasks(manifest.LastSync)
	if err != nil {
		return "", fmt.Errorf("failed to scan Claude tasks: %w", err)
	}

	// Reconcile any changes into manifest (also removes completed tasks)
	ReconcileTasks(manifest, sessionTasks)

	// Generate new session ID
	newSessionID := generateSessionID()

	// Hydrate new session with only active tasks (relay-only approach)
	activeTasks := manifest.ActiveTasks()
	if len(activeTasks) > 0 {
		if err := HydrateClaudeStore(newSessionID, activeTasks); err != nil {
			return newSessionID, fmt.Errorf("failed to hydrate tasks: %w", err)
		}
	}

	// Update manifest with new session
	manifest.LastSessionID = newSessionID
	if err := SaveManifest(projectPath, manifest); err != nil {
		return newSessionID, fmt.Errorf("failed to save manifest: %w", err)
	}

	// Cleanup old session directories (older than 24 hours)
	CleanupOldSessions(24 * time.Hour)

	return newSessionID, nil
}

// SyncOnHook performs lightweight task hydration for SessionStart hook
// This is faster than SyncOnLaunch - it just copies active tasks without full reconciliation
func SyncOnHook(projectPath, newSessionID string) error {
	// Check if this is an EDI project
	ediDir := filepath.Join(projectPath, ".edi")
	if _, err := os.Stat(ediDir); os.IsNotExist(err) {
		return nil // Not an EDI project
	}

	// Load manifest
	manifest, err := LoadManifest(projectPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Get only active tasks (relay-only approach)
	activeTasks := manifest.ActiveTasks()

	// Skip if no active tasks
	if len(activeTasks) == 0 {
		return nil
	}

	// Check if this session already has tasks (avoid duplicate hydration)
	home, _ := os.UserHomeDir()
	sessionPath := filepath.Join(home, ".claude", "tasks", newSessionID)
	if entries, _ := os.ReadDir(sessionPath); len(entries) > 0 {
		return nil // Session already has tasks
	}

	// Hydrate new session with only active tasks from manifest
	if err := HydrateClaudeStore(newSessionID, activeTasks); err != nil {
		return fmt.Errorf("failed to hydrate tasks: %w", err)
	}

	// Update manifest's last session ID
	manifest.LastSessionID = newSessionID
	if err := SaveManifest(projectPath, manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

// generateSessionID creates a new UUID session ID
func generateSessionID() string {
	return uuid.New().String()
}

// GetCurrentSessionID attempts to detect the current Claude session ID
// by finding the most recently modified session directory
func GetCurrentSessionID() (string, error) {
	sessions, err := ScanClaudeSessions()
	if err != nil {
		return "", err
	}

	if len(sessions) == 0 {
		return "", fmt.Errorf("no Claude sessions found")
	}

	return sessions[0].ID, nil
}

// CleanupOldSessions removes Claude task session directories older than maxAge
// This prevents unbounded growth of orphaned session directories
func CleanupOldSessions(maxAge time.Duration) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	tasksDir := filepath.Join(home, ".claude", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Only remove directories older than cutoff
		if info.ModTime().Before(cutoff) {
			sessionPath := filepath.Join(tasksDir, entry.Name())
			if err := os.RemoveAll(sessionPath); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}
