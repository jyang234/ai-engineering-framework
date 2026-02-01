// Package recall provides a public API for the Codex knowledge base.
// It wraps the internal SearchEngine behind a stable interface suitable
// for cross-module consumption (e.g., the EDI CLI).
package recall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/storage"
	"github.com/google/uuid"
)

// Client provides read/write access to the Codex knowledge base.
type Client struct {
	engine   *core.SearchEngine
	metadata *storage.MetadataStore
}

// NewClient creates a Client backed by the Codex SearchEngine.
// If embedding is unavailable the client falls back to FTS-only search.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	dbPath := expandHome(cfg.MetadataDBPath)

	engineCfg := core.Config{
		MetadataDBPath:      dbPath,
		LocalEmbeddingURL:   cfg.LocalEmbeddingURL,
		LocalEmbeddingModel: cfg.LocalEmbeddingModel,
	}

	engine, err := core.NewSearchEngine(ctx, engineCfg)
	if err != nil {
		return nil, fmt.Errorf("recall: open engine: %w", err)
	}

	// Open a separate metadata handle for FindByTitle (keyword search on title column)
	meta, err := storage.NewMetadataStore(dbPath)
	if err != nil {
		engine.Close()
		return nil, fmt.Errorf("recall: open metadata: %w", err)
	}

	return &Client{engine: engine, metadata: meta}, nil
}

// NewFTSOnlyClient creates a Client that only uses FTS keyword search,
// skipping the embedding/vector pipeline entirely. This is useful when
// Ollama is not available (e.g., CLI commands like `edi recall add`).
func NewFTSOnlyClient(cfg Config) (*Client, error) {
	dbPath := expandHome(cfg.MetadataDBPath)

	meta, err := storage.NewMetadataStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("recall: open metadata: %w", err)
	}

	return &Client{engine: nil, metadata: meta}, nil
}

// Search queries the knowledge base. When the full engine is available it uses
// hybrid vector+keyword search; otherwise falls back to FTS-only.
func (c *Client) Search(ctx context.Context, query string, opts SearchOpts) ([]Item, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	// If engine is available, use hybrid search
	if c.engine != nil {
		results, err := c.engine.Search(ctx, core.SearchRequest{
			Query: query,
			Types: opts.Types,
			Scope: opts.Scope,
			Limit: limit,
		})
		if err != nil {
			// Fall through to FTS-only on embedding errors
			if c.metadata != nil {
				return c.ftsSearch(query, opts, limit)
			}
			return nil, err
		}

		items := make([]Item, len(results))
		for i, r := range results {
			items[i] = itemFromCore(&r.Item)
		}
		return items, nil
	}

	// FTS-only fallback
	return c.ftsSearch(query, opts, limit)
}

func (c *Client) ftsSearch(query string, opts SearchOpts, limit int) ([]Item, error) {
	kwResults, err := c.metadata.KeywordSearch(query, limit*3)
	if err != nil {
		return nil, fmt.Errorf("recall: keyword search: %w", err)
	}

	var items []Item
	typeSet := make(map[string]bool, len(opts.Types))
	for _, t := range opts.Types {
		typeSet[t] = true
	}

	for _, kw := range kwResults {
		if len(typeSet) > 0 && !typeSet[kw.Type] {
			continue
		}
		if opts.Scope != "" && opts.Scope != "all" && kw.Scope != opts.Scope {
			continue
		}
		items = append(items, Item{
			ID:      kw.ID,
			Type:    kw.Type,
			Title:   kw.Title,
			Content: kw.Content,
			Tags:    kw.Tags,
			Scope:   kw.Scope,
		})
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

// Add adds a new item. If the engine is available, it also generates embeddings.
func (c *Client) Add(ctx context.Context, item Item) (string, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}

	coreItem := itemToCore(&item)

	if c.engine != nil {
		if err := c.engine.Add(ctx, coreItem); err != nil {
			return "", fmt.Errorf("recall: add (engine): %w", err)
		}
		return item.ID, nil
	}

	// FTS-only: save metadata directly
	if err := c.metadata.SaveItem(coreItemToRecord(coreItem)); err != nil {
		return "", fmt.Errorf("recall: add (metadata): %w", err)
	}
	return item.ID, nil
}

// Get retrieves an item by ID.
func (c *Client) Get(ctx context.Context, id string) (*Item, error) {
	record, err := c.metadata.GetItem(id)
	if err != nil {
		return nil, fmt.Errorf("recall: get: %w", err)
	}
	item := itemFromRecord(record)
	return &item, nil
}

// FindByTitle returns the first item with an exact title match, or nil.
func (c *Client) FindByTitle(ctx context.Context, title string) (*Item, error) {
	// Use keyword search + exact filter (MetadataStore has no direct FindByTitle)
	results, err := c.metadata.KeywordSearch(title, 50)
	if err != nil {
		return nil, fmt.Errorf("recall: find by title: %w", err)
	}
	for _, r := range results {
		if r.Title == title {
			item := Item{
				ID:      r.ID,
				Type:    r.Type,
				Title:   r.Title,
				Content: r.Content,
				Tags:    r.Tags,
				Scope:   r.Scope,
			}
			return &item, nil
		}
	}
	return nil, nil
}

// Close releases all resources.
func (c *Client) Close() error {
	var errs []string
	if c.engine != nil {
		if err := c.engine.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.metadata != nil {
		if err := c.metadata.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("recall: close: %s", strings.Join(errs, "; "))
	}
	return nil
}

// --- conversion helpers ---

func itemFromCore(ci *core.Item) Item {
	return Item{
		ID:        ci.ID,
		Type:      ci.Type,
		Title:     ci.Title,
		Content:   ci.Content,
		Tags:      ci.Tags,
		Scope:     ci.Scope,
		Source:    ci.Source,
		Metadata:  ci.Metadata,
		CreatedAt: ci.CreatedAt,
		UpdatedAt: ci.UpdatedAt,
	}
}

func itemToCore(i *Item) *core.Item {
	return &core.Item{
		ID:        i.ID,
		Type:      i.Type,
		Title:     i.Title,
		Content:   i.Content,
		Tags:      i.Tags,
		Scope:     i.Scope,
		Source:    i.Source,
		Metadata:  i.Metadata,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

func itemFromRecord(r *storage.ItemRecord) Item {
	return Item{
		ID:        r.ID,
		Type:      r.Type,
		Title:     r.Title,
		Content:   r.Content,
		Tags:      r.Tags,
		Scope:     r.Scope,
		Source:    r.Source,
		Metadata:  r.Metadata,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func coreItemToRecord(i *core.Item) *storage.ItemRecord {
	return &storage.ItemRecord{
		ID:        i.ID,
		Type:      i.Type,
		Title:     i.Title,
		Content:   i.Content,
		Tags:      i.Tags,
		Scope:     i.Scope,
		Source:    i.Source,
		Metadata:  i.Metadata,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.Getwd() // fallback
		if h, err2 := os.UserHomeDir(); err2 == nil {
			home = h
		} else if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
