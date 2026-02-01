package recall

import (
	"context"

	pkgrecall "github.com/anthropics/aef/codex/pkg/recall"
)

// Backend is the interface that both v0 Storage and Codex satisfy
// for the `edi recall` CLI commands.
type Backend interface {
	Search(query string, types []string, scope string, limit int) ([]Item, error)
	Add(item *Item) error
	FindByTitle(title string) (*Item, error)
	Close() error
}

// Compile-time check: Storage satisfies Backend.
var _ Backend = (*Storage)(nil)

// --- Codex adapter ---

// CodexBackend adapts codex/pkg/recall.Client to the Backend interface.
type CodexBackend struct {
	client *pkgrecall.Client
}

// NewCodexBackend creates a Backend backed by the Codex engine.
// Set ftsOnly=true to skip embedding (suitable for CLI add/search without Ollama).
func NewCodexBackend(dbPath string, ftsOnly bool) (Backend, error) {
	cfg := pkgrecall.Config{MetadataDBPath: dbPath}

	var client *pkgrecall.Client
	var err error
	if ftsOnly {
		client, err = pkgrecall.NewFTSOnlyClient(cfg)
	} else {
		client, err = pkgrecall.NewClient(context.Background(), cfg)
	}
	if err != nil {
		return nil, err
	}
	return &CodexBackend{client: client}, nil
}

func (b *CodexBackend) Search(query string, types []string, scope string, limit int) ([]Item, error) {
	results, err := b.client.Search(context.Background(), query, pkgrecall.SearchOpts{
		Types: types,
		Scope: scope,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, len(results))
	for i, r := range results {
		items[i] = Item{
			ID:        r.ID,
			Type:      r.Type,
			Title:     r.Title,
			Content:   r.Content,
			Tags:      r.Tags,
			Scope:     r.Scope,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		}
	}
	return items, nil
}

func (b *CodexBackend) Add(item *Item) error {
	_, err := b.client.Add(context.Background(), pkgrecall.Item{
		ID:        item.ID,
		Type:      item.Type,
		Title:     item.Title,
		Content:   item.Content,
		Tags:      item.Tags,
		Scope:     item.Scope,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	})
	return err
}

func (b *CodexBackend) FindByTitle(title string) (*Item, error) {
	result, err := b.client.FindByTitle(context.Background(), title)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &Item{
		ID:        result.ID,
		Type:      result.Type,
		Title:     result.Title,
		Content:   result.Content,
		Tags:      result.Tags,
		Scope:     result.Scope,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
	}, nil
}

func (b *CodexBackend) Close() error {
	return b.client.Close()
}
