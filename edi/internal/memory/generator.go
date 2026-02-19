package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/edi/internal/config"
	"github.com/anthropics/aef/edi/internal/recall"
)

// SlotBudget defines the maximum number of promoted items per type.
const (
	MaxPatterns  = 10
	MaxFailures  = 10
	MaxDecisions = 10
)

// MaxMemoryLines is the hard limit on MEMORY.md line count.
// Claude Code truncates lines after 200 in the system prompt.
const MaxMemoryLines = 195

// ediManagedHeadings lists section headings that EDI generates and manages.
// Sections with these headings are replaced on each generation.
// All other sections (e.g., Claude's auto-written memories) are preserved.
var ediManagedHeadings = map[string]bool{
	"Project Quick Reference": true,
	"Current State":           true,
	"Key Patterns":            true,
	"Known Pitfalls":          true,
	"Key Decisions":           true,
	"Topic Index":             true,
}

// PromotionCriteria defines what makes a RECALL item eligible for MEMORY.md.
type PromotionCriteria struct {
	MinUsefulnessScore float64
	MinRetrievalCount  int
	MaxAgeDays         int
}

// DefaultPromotionCriteria returns conservative promotion thresholds.
func DefaultPromotionCriteria() PromotionCriteria {
	return PromotionCriteria{
		MinUsefulnessScore: 2.0, // Accumulated score (not rate — matches v0 scoring)
		MinRetrievalCount:  3,
		MaxAgeDays:         90,
	}
}

// Generate creates the EDI-managed portion of MEMORY.md content.
// It assembles: project quick reference, current state, promoted RECALL items,
// and a topic index. It does NOT include sections that Claude manages
// (like EDI Observations or user-added sections).
func Generate(cfg *config.Config, projectPath string) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Project Memory\n\n")

	// Section 1: Project Quick Reference (from .edi/profile.md)
	if cfg.Briefing.IncludeProfile {
		if profile, err := loadProfile(projectPath); err == nil && profile != "" {
			sb.WriteString("## Project Quick Reference\n")
			sb.WriteString(condense(profile, 30))
			sb.WriteString("\n\n")
		}
	}

	// Section 2: Current State (from .edi/status.md)
	if cfg.Briefing.IncludeStatus {
		if status, err := loadStatus(projectPath); err == nil && status != "" {
			sb.WriteString("## Current State\n")
			sb.WriteString(status)
			sb.WriteString("\n\n")
		}
	}

	// Sections 3-5: Promoted RECALL items
	if cfg.Recall.Enabled {
		promoted, err := getPromotedItems(cfg, projectPath)
		if err == nil && len(promoted) > 0 {
			sb.WriteString(renderPromotedItems(promoted))
		}
	}

	// Section 6: Topic index (links to topic files in memory dir)
	memDir := DetectAutoMemoryDir(projectPath)
	if memDir != "" {
		if topics := listTopicFiles(memDir); len(topics) > 0 {
			sb.WriteString("## Topic Index\n")
			for _, t := range topics {
				sb.WriteString(fmt.Sprintf("- [%s](%s)\n", t.name, t.file))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// WriteMemoryFile generates EDI content and merges it with existing MEMORY.md,
// preserving any sections that Claude or the user wrote (like EDI Observations
// and auto-saved memories). Returns the path written.
func WriteMemoryFile(cfg *config.Config, projectPath string) (string, error) {
	memPath := MemoryFilePath(projectPath)
	if memPath == "" {
		return "", fmt.Errorf("could not resolve auto memory directory for %s", projectPath)
	}

	ediContent, err := Generate(cfg, projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to generate memory content: %w", err)
	}

	// Read existing MEMORY.md to preserve non-EDI sections
	var result string
	if existing, err := os.ReadFile(memPath); err == nil && len(existing) > 0 {
		result = mergeWithExisting(ediContent, string(existing))
	} else {
		// First generation — add EDI Observations placeholder
		result = ediContent + ediObservationsPlaceholder()
	}

	// Enforce line budget on the final merged content
	result = enforceLineBudget(result, MaxMemoryLines)

	if err := os.WriteFile(memPath, []byte(result), 0644); err != nil {
		return "", fmt.Errorf("failed to write MEMORY.md: %w", err)
	}

	return memPath, nil
}

// SeedFromProfile creates an initial MEMORY.md from the project profile.
// Used during `edi init` to bootstrap auto memory.
func SeedFromProfile(projectPath string) (string, error) {
	memPath := MemoryFilePath(projectPath)
	if memPath == "" {
		return "", fmt.Errorf("could not resolve auto memory directory for %s", projectPath)
	}

	// Don't overwrite existing MEMORY.md
	if _, err := os.Stat(memPath); err == nil {
		return memPath, nil
	}

	var sb strings.Builder
	sb.WriteString("# Project Memory\n\n")

	if profile, err := loadProfile(projectPath); err == nil && profile != "" {
		sb.WriteString("## Project Quick Reference\n")
		sb.WriteString(condense(profile, 30))
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Key Patterns\n")
	sb.WriteString("<!-- Populated from RECALL as patterns are captured and proven useful -->\n\n")

	sb.WriteString("## Known Pitfalls\n")
	sb.WriteString("<!-- Populated from RECALL as failures are captured -->\n\n")

	sb.WriteString("## Key Decisions\n")
	sb.WriteString("<!-- Populated from RECALL as decisions are captured -->\n\n")

	sb.WriteString(ediObservationsPlaceholder())

	if err := os.WriteFile(memPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write initial MEMORY.md: %w", err)
	}

	return memPath, nil
}

// --- merge logic ---

// memorySection represents a parsed section of MEMORY.md.
type memorySection struct {
	heading string // The ## heading text (without "## " prefix)
	body    string // Everything after the heading line until next ## or EOF
}

// mergeWithExisting combines EDI-generated content with preserved sections
// from the existing MEMORY.md. EDI-managed sections (Project Quick Reference,
// Current State, Key Patterns, Known Pitfalls, Key Decisions, Topic Index)
// are replaced with freshly generated content. All other sections — including
// Claude's auto-written memories and EDI Observations — are preserved.
func mergeWithExisting(ediContent, existing string) string {
	preserved := extractPreservedSections(existing)

	var sb strings.Builder
	sb.WriteString(strings.TrimRight(ediContent, "\n"))
	sb.WriteString("\n")

	hasObservations := false
	for _, section := range preserved {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("## %s\n", section.heading))
		sb.WriteString(section.body)
		if !strings.HasSuffix(section.body, "\n") {
			sb.WriteString("\n")
		}
		if section.heading == "EDI Observations" {
			hasObservations = true
		}
	}

	// Ensure EDI Observations section always exists
	if !hasObservations {
		sb.WriteString("\n")
		sb.WriteString(ediObservationsPlaceholder())
	}

	return sb.String()
}

// extractPreservedSections returns sections from existing MEMORY.md that
// are NOT EDI-managed — these are preserved across regenerations.
func extractPreservedSections(content string) []memorySection {
	sections := parseMemorySections(content)
	var preserved []memorySection
	for _, s := range sections {
		if !ediManagedHeadings[s.heading] {
			preserved = append(preserved, s)
		}
	}
	return preserved
}

// parseMemorySections splits MEMORY.md content into sections by ## headers.
// Content before the first ## header (typically the "# Project Memory" line)
// is not included in the returned sections.
func parseMemorySections(content string) []memorySection {
	var sections []memorySection
	lines := strings.Split(content, "\n")

	var current *memorySection
	var bodyLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if current != nil {
				current.body = strings.Join(bodyLines, "\n")
				sections = append(sections, *current)
			}
			heading := strings.TrimPrefix(line, "## ")
			current = &memorySection{heading: heading}
			bodyLines = nil
		} else if current != nil {
			bodyLines = append(bodyLines, line)
		}
		// Lines before the first ## are the file header — ignored
	}

	// Save last section
	if current != nil {
		current.body = strings.Join(bodyLines, "\n")
		sections = append(sections, *current)
	}

	return sections
}

// ediObservationsPlaceholder returns the default EDI Observations section.
func ediObservationsPlaceholder() string {
	return "## EDI Observations\n" +
		"<!-- EDI may add concurrence/dissent notes here during sessions -->\n" +
		"<!-- This section is re-evaluated each session -->\n\n"
}

// --- internal helpers ---

func loadProfile(projectPath string) (string, error) {
	profilePath := filepath.Join(projectPath, ".edi", "profile.md")
	content, err := os.ReadFile(profilePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func loadStatus(projectPath string) (string, error) {
	statusPath := filepath.Join(projectPath, ".edi", "status.md")
	content, err := os.ReadFile(statusPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

// condense trims content to at most maxLines lines, preserving complete lines.
func condense(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

// enforceLineBudget trims output to maxLines, adding a note if truncated.
func enforceLineBudget(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	truncated := lines[:maxLines-1]
	truncated = append(truncated, "<!-- Truncated to fit 200-line auto memory budget -->")
	return strings.Join(truncated, "\n")
}

// getPromotedItems retrieves RECALL items eligible for MEMORY.md promotion.
// projectPath is reserved for future per-project scope filtering.
func getPromotedItems(cfg *config.Config, _ string) ([]recall.Item, error) {
	backend, err := openBackend(cfg)
	if err != nil {
		return nil, err
	}
	defer backend.Close()

	criteria := DefaultPromotionCriteria()
	cutoff := time.Now().AddDate(0, 0, -criteria.MaxAgeDays)

	var promoted []recall.Item

	// Fetch top items by type
	for _, itemType := range []string{"pattern", "decision", "failure"} {
		limit := MaxPatterns
		if itemType == "decision" {
			limit = MaxDecisions
		} else if itemType == "failure" {
			limit = MaxFailures
		}

		items, err := backend.Search("*", []string{itemType}, "all", limit*2)
		if err != nil {
			continue
		}

		count := 0
		for _, item := range items {
			if count >= limit {
				break
			}
			// Apply promotion criteria
			if item.CreatedAt.Before(cutoff) {
				continue
			}
			if item.UsefulnessScore < criteria.MinUsefulnessScore &&
				item.UseCount < criteria.MinRetrievalCount {
				// Items that are both low-score and low-retrieval don't qualify
				// But allow items that meet either threshold
				continue
			}
			promoted = append(promoted, item)
			count++
		}
	}

	return promoted, nil
}

// renderPromotedItems formats promoted RECALL items into markdown sections.
func renderPromotedItems(items []recall.Item) string {
	var sb strings.Builder

	patterns := filterByType(items, "pattern")
	failures := filterByType(items, "failure")
	decisions := filterByType(items, "decision")

	if len(patterns) > 0 {
		sb.WriteString("## Key Patterns\n")
		sb.WriteString("<!-- Promoted from RECALL — high usefulness items -->\n")
		for _, p := range patterns {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", p.Title, firstSentence(p.Content)))
		}
		sb.WriteString("\n")
	}

	if len(failures) > 0 {
		sb.WriteString("## Known Pitfalls\n")
		sb.WriteString("<!-- Promoted from RECALL — captured failures -->\n")
		for _, f := range failures {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", f.Title, firstSentence(f.Content)))
		}
		sb.WriteString("\n")
	}

	if len(decisions) > 0 {
		sb.WriteString("## Key Decisions\n")
		sb.WriteString("<!-- Promoted from RECALL — architectural decisions -->\n")
		for _, d := range decisions {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", d.Title, firstSentence(d.Content)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func filterByType(items []recall.Item, itemType string) []recall.Item {
	var result []recall.Item
	for _, item := range items {
		if item.Type == itemType {
			result = append(result, item)
		}
	}
	return result
}

// firstSentence extracts the first sentence or line of content for summary.
func firstSentence(content string) string {
	// Strip markdown headers
	for strings.HasPrefix(content, "#") {
		idx := strings.Index(content, "\n")
		if idx < 0 {
			break
		}
		content = strings.TrimSpace(content[idx+1:])
	}

	// Take first line
	if idx := strings.Index(content, "\n"); idx >= 0 {
		content = content[:idx]
	}

	// Truncate if too long
	if len(content) > 120 {
		content = content[:117] + "..."
	}

	return strings.TrimSpace(content)
}

type topicFile struct {
	name string
	file string
}

func listTopicFiles(memDir string) []topicFile {
	entries, err := os.ReadDir(memDir)
	if err != nil {
		return nil
	}

	var topics []topicFile
	for _, e := range entries {
		if e.IsDir() || e.Name() == "MEMORY.md" {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			name := strings.TrimSuffix(e.Name(), ".md")
			name = strings.ReplaceAll(name, "-", " ")
			topics = append(topics, topicFile{name: name, file: e.Name()})
		}
	}
	return topics
}

// openBackend opens the appropriate RECALL backend for reading.
func openBackend(cfg *config.Config) (recall.Backend, error) {
	if cfg.Recall.Backend == "codex" {
		dbPath := config.ResolvePath(cfg.Codex.MetadataDB)
		if dbPath == "" {
			home, _ := os.UserHomeDir()
			dbPath = filepath.Join(home, ".edi", "codex.db")
		}
		return recall.NewCodexBackend(dbPath, true)
	}

	// v0 backend
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve home directory: %w", err)
	}
	dbPath := filepath.Join(home, ".edi", "recall", "global.db")
	return recall.NewStorage(dbPath)
}
