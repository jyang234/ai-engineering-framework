package core

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/anthropics/aef/codex/internal/chunking"
	"github.com/anthropics/aef/codex/internal/storage"
)

// Common test errors
var (
	ErrMockEmbedding = errors.New("mock embedding error")
	ErrMockStorage   = errors.New("mock storage error")
	ErrMockChunking  = errors.New("mock chunking error")
)

// MockCodeEmbedder implements CodeEmbedder for testing
type MockCodeEmbedder struct {
	mu           sync.Mutex
	EmbedFunc    func(ctx context.Context, texts []string) ([]float32, error)
	QueryFunc    func(ctx context.Context, query string) ([]float32, error)
	CallCount    int
	LastTexts    []string
	FailOnCall   int // Fail on Nth call (0 = never fail)
	FixedVector  []float32
}

func NewMockCodeEmbedder() *MockCodeEmbedder {
	return &MockCodeEmbedder{
		FixedVector: make([]float32, 1024), // Voyage Code-3 dimension
	}
}

func (m *MockCodeEmbedder) EmbedCode(ctx context.Context, texts []string) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++
	m.LastTexts = texts

	if m.FailOnCall > 0 && m.CallCount >= m.FailOnCall {
		return nil, ErrMockEmbedding
	}

	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, texts)
	}

	return m.FixedVector, nil
}

func (m *MockCodeEmbedder) EmbedCodeQuery(ctx context.Context, query string) ([]float32, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query)
	}
	return m.FixedVector, nil
}

// MockDocEmbedder implements DocEmbedder for testing
type MockDocEmbedder struct {
	mu          sync.Mutex
	EmbedFunc   func(ctx context.Context, texts []string) ([][]float32, error)
	CallCount   int
	LastTexts   []string
	FailOnCall  int
	FixedVector []float32
}

func NewMockDocEmbedder() *MockDocEmbedder {
	return &MockDocEmbedder{
		FixedVector: make([]float32, 3072), // OpenAI text-embedding-3-large dimension
	}
}

func (m *MockDocEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++
	m.LastTexts = texts

	if m.FailOnCall > 0 && m.CallCount >= m.FailOnCall {
		return nil, ErrMockEmbedding
	}

	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, texts)
	}

	// Return one vector per input text
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = m.FixedVector
	}
	return result, nil
}

// MockVectorStorage implements VectorStorage for testing
type MockVectorStorage struct {
	mu            sync.Mutex
	Vectors       map[string][]float32
	UpsertFunc    func(ctx context.Context, itemID string, vector []float32) error
	SearchFunc    func(ctx context.Context, queryVec []float32, limit int) []storage.ScoredResult
	UpsertCount   int
	SearchCount   int
	FailOnUpsert  int
	FailOnSearch  bool
}

func NewMockVectorStorage() *MockVectorStorage {
	return &MockVectorStorage{
		Vectors: make(map[string][]float32),
	}
}

func (m *MockVectorStorage) Upsert(ctx context.Context, itemID string, vector []float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpsertCount++

	if m.FailOnUpsert > 0 && m.UpsertCount >= m.FailOnUpsert {
		return ErrMockStorage
	}

	if m.UpsertFunc != nil {
		return m.UpsertFunc(ctx, itemID, vector)
	}

	m.Vectors[itemID] = vector
	return nil
}

func (m *MockVectorStorage) Search(ctx context.Context, queryVec []float32, limit int) []storage.ScoredResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SearchCount++

	if m.FailOnSearch {
		return nil
	}

	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, queryVec, limit)
	}

	// Return stored items as search results
	var results []storage.ScoredResult
	for id := range m.Vectors {
		results = append(results, storage.ScoredResult{
			ID:    id,
			Score: 0.9,
		})
		if len(results) >= limit {
			break
		}
	}
	return results
}

func (m *MockVectorStorage) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Vectors, id)
	return nil
}

// MockKeywordSearcher implements KeywordSearcher for testing
type MockKeywordSearcher struct {
	mu           sync.Mutex
	SearchFunc   func(query string, limit int) ([]storage.KeywordResult, error)
	CallCount    int
	FailOnSearch bool
	Results      []storage.KeywordResult
}

func NewMockKeywordSearcher() *MockKeywordSearcher {
	return &MockKeywordSearcher{}
}

func (m *MockKeywordSearcher) KeywordSearch(query string, limit int) ([]storage.KeywordResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++

	if m.FailOnSearch {
		return nil, ErrMockStorage
	}

	if m.SearchFunc != nil {
		return m.SearchFunc(query, limit)
	}

	if m.Results != nil {
		return m.Results, nil
	}

	return nil, nil
}

// MockMetadataStorage implements MetadataStorage for testing
type MockMetadataStorage struct {
	mu             sync.Mutex
	Items          map[string]*storage.ItemRecord
	Feedback       []*storage.FeedbackRecord
	FlightRecorder []*storage.FlightRecorderRecord
	SaveFunc       func(item *storage.ItemRecord) error
	FailOnSave     int
	SaveCount      int
	Closed         bool
	FailOnList     bool
	FailOnDelete   bool
	FailOnStats    bool
}

func NewMockMetadataStorage() *MockMetadataStorage {
	return &MockMetadataStorage{
		Items: make(map[string]*storage.ItemRecord),
	}
}

func (m *MockMetadataStorage) SaveItem(item *storage.ItemRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveCount++

	if m.FailOnSave > 0 && m.SaveCount >= m.FailOnSave {
		return ErrMockStorage
	}

	if m.SaveFunc != nil {
		return m.SaveFunc(item)
	}

	m.Items[item.ID] = item
	return nil
}

func (m *MockMetadataStorage) GetItem(id string) (*storage.ItemRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.Items[id]
	if !ok {
		return nil, errors.New("item not found")
	}
	return item, nil
}

func (m *MockMetadataStorage) RecordFeedback(feedback *storage.FeedbackRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Feedback = append(m.Feedback, feedback)
	return nil
}

func (m *MockMetadataStorage) LogFlightRecorder(entry *storage.FlightRecorderRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.FlightRecorder = append(m.FlightRecorder, entry)
	return nil
}

func (m *MockMetadataStorage) GetFlightRecorderEntries(sessionID string) ([]*storage.FlightRecorderRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var entries []*storage.FlightRecorderRecord
	for _, e := range m.FlightRecorder {
		if e.SessionID == sessionID {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (m *MockMetadataStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Closed = true
	return nil
}

func (m *MockMetadataStorage) ListItems(itemType, scope string, limit, offset int) ([]*storage.ItemRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.FailOnList {
		return nil, ErrMockStorage
	}

	var result []*storage.ItemRecord
	for _, item := range m.Items {
		if itemType != "" && item.Type != itemType {
			continue
		}
		if scope != "" && item.Scope != scope {
			continue
		}
		result = append(result, item)
	}

	if offset >= len(result) {
		return []*storage.ItemRecord{}, nil
	}
	result = result[offset:]
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (m *MockMetadataStorage) DeleteItem(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.FailOnDelete {
		return ErrMockStorage
	}

	delete(m.Items, id)
	return nil
}

func (m *MockMetadataStorage) CountItemsByType() (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.FailOnStats {
		return nil, ErrMockStorage
	}

	counts := make(map[string]int)
	for _, item := range m.Items {
		counts[item.Type]++
	}
	return counts, nil
}

// MockCodeChunker implements CodeChunker for testing
type MockCodeChunker struct {
	mu         sync.Mutex
	ChunkFunc  func(content []byte, lang, filePath string) ([]chunking.CodeChunk, error)
	CallCount  int
	LastLang   string
	FailOnCall int
	Closed     bool
}

func NewMockCodeChunker() *MockCodeChunker {
	return &MockCodeChunker{}
}

func (m *MockCodeChunker) ChunkFile(content []byte, lang, filePath string) ([]chunking.CodeChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++
	m.LastLang = lang

	if m.FailOnCall > 0 && m.CallCount >= m.FailOnCall {
		return nil, ErrMockChunking
	}

	if m.ChunkFunc != nil {
		return m.ChunkFunc(content, lang, filePath)
	}

	return []chunking.CodeChunk{
		{
			Content:   string(content),
			Type:      "function",
			Name:      "mockFunction",
			StartLine: 1,
			EndLine:   10,
			FilePath:  filePath,
			Language:  lang,
		},
	}, nil
}

func (m *MockCodeChunker) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Closed = true
	return nil
}

// MockDocChunker implements DocChunker for testing
type MockDocChunker struct {
	mu         sync.Mutex
	ChunkFunc  func(ctx context.Context, content, filePath string) ([]chunking.DocChunk, error)
	CallCount  int
	FailOnCall int
}

func NewMockDocChunker() *MockDocChunker {
	return &MockDocChunker{}
}

func (m *MockDocChunker) ChunkDocument(ctx context.Context, content, filePath string) ([]chunking.DocChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++

	if m.FailOnCall > 0 && m.CallCount >= m.FailOnCall {
		return nil, ErrMockChunking
	}

	if m.ChunkFunc != nil {
		return m.ChunkFunc(ctx, content, filePath)
	}

	return []chunking.DocChunk{
		{
			OriginalContent: content,
			EnrichedContent: content,
			Section:         "Test Section",
			StartLine:       1,
			EndLine:         10,
		},
	}, nil
}

// MockIDGenerator implements IDGenerator for testing
type MockIDGenerator struct {
	mu      sync.Mutex
	Counter int
	Prefix  string
}

func NewMockIDGenerator(prefix string) *MockIDGenerator {
	return &MockIDGenerator{Prefix: prefix}
}

func (m *MockIDGenerator) GenerateID() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Counter++
	if m.Prefix != "" {
		return fmt.Sprintf("%s-%d", m.Prefix, m.Counter)
	}
	return fmt.Sprintf("mock-id-%d", m.Counter)
}
