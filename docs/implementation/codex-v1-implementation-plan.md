# Codex v1 Implementation Plan (RECALL Upgrade)

**Version**: 1.0  
**Created**: January 25, 2026  
**Target**: Claude Code execution  
**Prerequisites**: EDI Phase 2 complete (RECALL v0 working)  
**Estimated Duration**: 4 weeks

---

## Executive Summary

Codex v1 upgrades RECALL from SQLite FTS to production-grade hybrid retrieval with:
- **Qdrant** for vector + BM25 search
- **Voyage Code-3** for code embeddings
- **text-embedding-3-large** for doc embeddings
- **Multi-stage reranking** with self-hosted BGE models
- **AST-aware chunking** with Tree-sitter
- **Contextual retrieval** with Claude Haiku
- **Web UI** for browsing knowledge

### Expected Accuracy Improvement

| Metric | RECALL v0 (FTS) | Codex v1 (Hybrid) |
|--------|-----------------|-------------------|
| Top-10 recall | ~60% | ~85% |
| Retrieval failures | ~40% | ~13% |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          CODEX v1 ARCHITECTURE                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                     INGESTION PIPELINE                          │   │
│  │                                                                  │   │
│  │  Sources           Chunking            Embedding      Storage   │   │
│  │  ┌───────┐        ┌──────────┐        ┌─────────┐   ┌────────┐ │   │
│  │  │ Code  │──────► │ AST-aware│──────► │ Voyage  │──►│        │ │   │
│  │  │ Files │        │ (sitter) │        │ Code-3  │   │        │ │   │
│  │  └───────┘        └──────────┘        └─────────┘   │        │ │   │
│  │  ┌───────┐        ┌──────────┐        ┌─────────┐   │ Qdrant │ │   │
│  │  │ Docs  │──────► │Contextual│──────► │text-emb-│──►│ (Vec + │ │   │
│  │  │ MD    │        │ + Haiku  │        │ 3-large │   │  BM25) │ │   │
│  │  └───────┘        └──────────┘        └─────────┘   │        │ │   │
│  │  ┌───────┐                                          │        │ │   │
│  │  │Manual │─────────────────────────────────────────►│        │ │   │
│  │  │Items  │                                          └────────┘ │   │
│  │  └───────┘                                                     │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                     RETRIEVAL PIPELINE                          │   │
│  │                                                                  │   │
│  │  Query        Stage 1         Stage 2         Stage 3    Result │   │
│  │  ┌─────┐     ┌────────┐     ┌────────┐     ┌────────┐  ┌─────┐ │   │
│  │  │Query│────►│Qdrant  │────►│BGE-base│────►│BGE-v2  │─►│Top-K│ │   │
│  │  │     │     │Hybrid  │     │Rerank  │     │M3      │  │     │ │   │
│  │  └─────┘     │(50 res)│     │(20 res)│     │(10 res)│  └─────┘ │   │
│  │              └────────┘     └────────┘     └────────┘          │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │
│  │   MCP Server │  │   Web UI     │  │   CLI        │                  │
│  │   (for EDI)  │  │   (wiki)     │  │   (admin)    │                  │
│  └──────────────┘  └──────────────┘  └──────────────┘                  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Project Structure

```
codex/
├── cmd/
│   ├── recall-mcp/          # MCP server binary
│   │   └── main.go
│   ├── codex-web/           # Web UI binary
│   │   └── main.go
│   └── codex-cli/           # Admin CLI
│       └── main.go
├── internal/
│   ├── core/                # Core library
│   │   ├── search.go        # Search orchestration
│   │   ├── index.go         # Indexing orchestration
│   │   └── types.go
│   ├── storage/
│   │   ├── qdrant.go        # Qdrant client wrapper
│   │   └── sqlite.go        # Metadata storage
│   ├── chunking/
│   │   ├── ast.go           # Tree-sitter AST chunking
│   │   ├── contextual.go    # Haiku context generation
│   │   └── markdown.go      # Markdown-aware chunking
│   ├── embedding/
│   │   ├── voyage.go        # Voyage Code-3
│   │   └── openai.go        # text-embedding-3-large
│   ├── reranking/
│   │   ├── hugot.go         # ONNX inference via hugot
│   │   └── models.go        # Model management
│   ├── mcp/
│   │   ├── server.go        # MCP server setup
│   │   └── tools.go         # Tool handlers
│   └── web/
│       ├── server.go        # Gin web server
│       ├── handlers.go      # HTTP handlers
│       └── templates/       # HTML templates
├── web/
│   └── static/              # CSS, JS assets
├── models/                  # ONNX models (gitignored)
│   ├── bge-reranker-base/
│   └── bge-reranker-v2-m3/
├── go.mod
├── go.sum
└── Makefile
```

---

## Phase 1: Qdrant Integration (Week 1)

### Goal
Replace SQLite FTS with Qdrant hybrid search.

### 1.1 Qdrant Setup

**Task 1.1.1: Docker Compose for Qdrant**

File: `docker-compose.yml`
```yaml
version: '3.8'
services:
  qdrant:
    image: qdrant/qdrant:v1.9.0
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - qdrant_data:/qdrant/storage
    environment:
      - QDRANT__SERVICE__GRPC_PORT=6334
      - QDRANT__STORAGE__ON_DISK_PAYLOAD=true

volumes:
  qdrant_data:
```

**Task 1.1.2: Qdrant client wrapper**

File: `internal/storage/qdrant.go`
```go
package storage

import (
    "context"
    
    "github.com/qdrant/go-client/qdrant"
)

type QdrantStorage struct {
    client *qdrant.Client
    collection string
}

type SearchParams struct {
    Query       string
    QueryVector []float32
    Types       []string
    Scope       string
    Limit       int
    UseHybrid   bool
}

func NewQdrantStorage(addr, collection string) (*QdrantStorage, error) {
    client, err := qdrant.NewClient(&qdrant.Config{
        Addr: addr,
    })
    if err != nil {
        return nil, err
    }
    
    return &QdrantStorage{
        client:     client,
        collection: collection,
    }, nil
}

func (s *QdrantStorage) EnsureCollection(ctx context.Context, vectorSize uint64) error {
    exists, err := s.client.CollectionExists(ctx, s.collection)
    if err != nil {
        return err
    }
    
    if !exists {
        return s.client.CreateCollection(ctx, &qdrant.CreateCollection{
            CollectionName: s.collection,
            VectorsConfig: &qdrant.VectorsConfig{
                Config: &qdrant.VectorsConfig_Params{
                    Params: &qdrant.VectorParams{
                        Size:     vectorSize,
                        Distance: qdrant.Distance_Cosine,
                    },
                },
            },
            // Enable BM25 for hybrid search
            SparseVectorsConfig: map[string]*qdrant.SparseVectorParams{
                "bm25": {},
            },
        })
    }
    
    return nil
}

func (s *QdrantStorage) Upsert(ctx context.Context, points []*qdrant.PointStruct) error {
    _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
        CollectionName: s.collection,
        Points:         points,
    })
    return err
}

func (s *QdrantStorage) HybridSearch(ctx context.Context, params SearchParams) ([]SearchResult, error) {
    // Dense vector search
    denseSearch := &qdrant.QueryPoints{
        CollectionName: s.collection,
        Query:          qdrant.NewQuery(params.QueryVector...),
        Limit:          qdrant.PtrOf(uint64(params.Limit)),
        WithPayload:    qdrant.NewWithPayload(true),
    }
    
    // Sparse BM25 search
    sparseSearch := &qdrant.QueryPoints{
        CollectionName: s.collection,
        Query: &qdrant.Query{
            Query: &qdrant.Query_Nearest{
                Nearest: &qdrant.VectorInput{
                    Variant: &qdrant.VectorInput_Document{
                        Document: &qdrant.Document{
                            Text: params.Query,
                        },
                    },
                },
            },
        },
        Using:       qdrant.PtrOf("bm25"),
        Limit:       qdrant.PtrOf(uint64(params.Limit)),
        WithPayload: qdrant.NewWithPayload(true),
    }
    
    // Execute both and merge with RRF
    denseResults, err := s.client.Query(ctx, denseSearch)
    if err != nil {
        return nil, err
    }
    
    sparseResults, err := s.client.Query(ctx, sparseSearch)
    if err != nil {
        return nil, err
    }
    
    return reciprocalRankFusion(denseResults, sparseResults, params.Limit), nil
}

func reciprocalRankFusion(dense, sparse []*qdrant.ScoredPoint, limit int) []SearchResult {
    k := 60.0 // RRF constant
    scores := make(map[string]float64)
    points := make(map[string]*qdrant.ScoredPoint)
    
    for rank, p := range dense {
        id := p.Id.GetUuid()
        scores[id] += 1.0 / (k + float64(rank+1))
        points[id] = p
    }
    
    for rank, p := range sparse {
        id := p.Id.GetUuid()
        scores[id] += 1.0 / (k + float64(rank+1))
        if _, exists := points[id]; !exists {
            points[id] = p
        }
    }
    
    // Sort by fused score
    // ... implementation ...
    
    return results[:min(len(results), limit)]
}
```

### 1.2 Embedding Services

**Task 1.2.1: Voyage Code-3 client**

File: `internal/embedding/voyage.go`
```go
package embedding

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "os"
)

type VoyageClient struct {
    apiKey  string
    baseURL string
}

type VoyageRequest struct {
    Input     []string `json:"input"`
    Model     string   `json:"model"`
    InputType string   `json:"input_type"`
}

type VoyageResponse struct {
    Data []struct {
        Embedding []float32 `json:"embedding"`
    } `json:"data"`
}

func NewVoyageClient() *VoyageClient {
    return &VoyageClient{
        apiKey:  os.Getenv("VOYAGE_API_KEY"),
        baseURL: "https://api.voyageai.com/v1/embeddings",
    }
}

func (c *VoyageClient) EmbedCode(ctx context.Context, texts []string) ([][]float32, error) {
    return c.embed(ctx, texts, "voyage-code-3", "document")
}

func (c *VoyageClient) EmbedCodeQuery(ctx context.Context, query string) ([]float32, error) {
    embeddings, err := c.embed(ctx, []string{query}, "voyage-code-3", "query")
    if err != nil {
        return nil, err
    }
    return embeddings[0], nil
}

func (c *VoyageClient) embed(ctx context.Context, texts []string, model, inputType string) ([][]float32, error) {
    req := VoyageRequest{
        Input:     texts,
        Model:     model,
        InputType: inputType,
    }
    
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
    httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var voyageResp VoyageResponse
    if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
        return nil, err
    }
    
    embeddings := make([][]float32, len(voyageResp.Data))
    for i, d := range voyageResp.Data {
        embeddings[i] = d.Embedding
    }
    
    return embeddings, nil
}
```

**Task 1.2.2: OpenAI embeddings client**

File: `internal/embedding/openai.go`
```go
package embedding

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "os"
)

type OpenAIClient struct {
    apiKey  string
    baseURL string
}

func NewOpenAIClient() *OpenAIClient {
    return &OpenAIClient{
        apiKey:  os.Getenv("OPENAI_API_KEY"),
        baseURL: "https://api.openai.com/v1/embeddings",
    }
}

func (c *OpenAIClient) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
    return c.embed(ctx, texts, "text-embedding-3-large")
}

func (c *OpenAIClient) embed(ctx context.Context, texts []string, model string) ([][]float32, error) {
    req := map[string]interface{}{
        "input": texts,
        "model": model,
    }
    
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
    httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result struct {
        Data []struct {
            Embedding []float32 `json:"embedding"`
        } `json:"data"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    embeddings := make([][]float32, len(result.Data))
    for i, d := range result.Data {
        embeddings[i] = d.Embedding
    }
    
    return embeddings, nil
}
```

### 1.3 Migration from v0

**Task 1.3.1: Migrate existing items to Qdrant**

File: `internal/core/migrate.go`
```go
package core

import (
    "context"
    "database/sql"
    "log"
    
    "github.com/[user]/codex/internal/embedding"
    "github.com/[user]/codex/internal/storage"
)

func MigrateV0ToV1(ctx context.Context, sqliteDB *sql.DB, qdrant *storage.QdrantStorage) error {
    voyage := embedding.NewVoyageClient()
    openai := embedding.NewOpenAIClient()
    
    // Fetch all items from SQLite
    rows, err := sqliteDB.Query(`SELECT id, type, title, content, tags, scope FROM items`)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    var batch []Item
    for rows.Next() {
        var item Item
        rows.Scan(&item.ID, &item.Type, &item.Title, &item.Content, &item.Tags, &item.Scope)
        batch = append(batch, item)
        
        if len(batch) >= 100 {
            if err := indexBatch(ctx, batch, voyage, openai, qdrant); err != nil {
                log.Printf("Batch error: %v", err)
            }
            batch = batch[:0]
        }
    }
    
    // Final batch
    if len(batch) > 0 {
        return indexBatch(ctx, batch, voyage, openai, qdrant)
    }
    
    return nil
}

func indexBatch(ctx context.Context, items []Item, voyage *embedding.VoyageClient, openai *embedding.OpenAIClient, qdrant *storage.QdrantStorage) error {
    // Separate code and doc items
    var codeTexts, docTexts []string
    var codeItems, docItems []Item
    
    for _, item := range items {
        if item.Type == "pattern" || item.Type == "failure" {
            codeTexts = append(codeTexts, item.Content)
            codeItems = append(codeItems, item)
        } else {
            docTexts = append(docTexts, item.Content)
            docItems = append(docItems, item)
        }
    }
    
    // Embed code with Voyage
    if len(codeTexts) > 0 {
        embeddings, err := voyage.EmbedCode(ctx, codeTexts)
        if err != nil {
            return err
        }
        // Upsert to Qdrant...
    }
    
    // Embed docs with OpenAI
    if len(docTexts) > 0 {
        embeddings, err := openai.EmbedDocuments(ctx, docTexts)
        if err != nil {
            return err
        }
        // Upsert to Qdrant...
    }
    
    return nil
}
```

### 1.4 Validation Checkpoint

**Acceptance Criteria for Phase 1:**
- [ ] Qdrant running via Docker Compose
- [ ] Voyage embeddings working for code
- [ ] OpenAI embeddings working for docs
- [ ] Hybrid search returns better results than FTS
- [ ] Migration script moves v0 items to Qdrant

---

## Phase 2: Reranking Pipeline (Week 2)

### Goal
Add multi-stage reranking with self-hosted BGE models.

### 2.1 Hugot Integration

**Task 2.1.1: Setup ONNX models**

```bash
# Download pre-converted ONNX models
mkdir -p models
cd models

# BGE Reranker Base (Stage 1)
wget https://huggingface.co/BAAI/bge-reranker-base/resolve/main/onnx/model.onnx \
     -O bge-reranker-base/model.onnx

# BGE Reranker v2 M3 (Stage 2)
wget https://huggingface.co/BAAI/bge-reranker-v2-m3/resolve/main/onnx/model.onnx \
     -O bge-reranker-v2-m3/model.onnx
```

**Task 2.1.2: Implement reranker**

File: `internal/reranking/hugot.go`
```go
package reranking

import (
    "sort"
    
    "github.com/knights-analytics/hugot"
    "github.com/knights-analytics/hugot/pipelines"
)

type Reranker struct {
    session    *hugot.Session
    stage1     *pipelines.TextClassificationPipeline
    stage2     *pipelines.TextClassificationPipeline
}

type RerankResult struct {
    ID    string
    Score float64
}

func NewReranker(modelsPath string) (*Reranker, error) {
    session, err := hugot.NewSession()
    if err != nil {
        return nil, err
    }
    
    // Load Stage 1: BGE Reranker Base
    stage1Config := pipelines.TextClassificationConfig{
        ModelPath: modelsPath + "/bge-reranker-base",
    }
    stage1, err := pipelines.NewTextClassificationPipeline(session, stage1Config)
    if err != nil {
        return nil, err
    }
    
    // Load Stage 2: BGE Reranker v2 M3
    stage2Config := pipelines.TextClassificationConfig{
        ModelPath: modelsPath + "/bge-reranker-v2-m3",
    }
    stage2, err := pipelines.NewTextClassificationPipeline(session, stage2Config)
    if err != nil {
        return nil, err
    }
    
    return &Reranker{
        session: session,
        stage1:  stage1,
        stage2:  stage2,
    }, nil
}

func (r *Reranker) Rerank(query string, documents []Document, limit int) ([]RerankResult, error) {
    // Stage 1: Fast rerank with BGE-base (50 -> 20)
    stage1Results := r.rerankWithModel(r.stage1, query, documents)
    sort.Slice(stage1Results, func(i, j int) bool {
        return stage1Results[i].Score > stage1Results[j].Score
    })
    
    // Take top 20 for stage 2
    top20 := stage1Results
    if len(top20) > 20 {
        top20 = top20[:20]
    }
    
    // Stage 2: Precise rerank with BGE-v2-m3 (20 -> 10)
    var top20Docs []Document
    for _, r := range top20 {
        for _, d := range documents {
            if d.ID == r.ID {
                top20Docs = append(top20Docs, d)
                break
            }
        }
    }
    
    stage2Results := r.rerankWithModel(r.stage2, query, top20Docs)
    sort.Slice(stage2Results, func(i, j int) bool {
        return stage2Results[i].Score > stage2Results[j].Score
    })
    
    if len(stage2Results) > limit {
        stage2Results = stage2Results[:limit]
    }
    
    return stage2Results, nil
}

func (r *Reranker) rerankWithModel(model *pipelines.TextClassificationPipeline, query string, docs []Document) []RerankResult {
    results := make([]RerankResult, len(docs))
    
    for i, doc := range docs {
        // Prepare query-document pair
        input := query + " [SEP] " + doc.Content
        
        output, _ := model.Run([]string{input})
        
        results[i] = RerankResult{
            ID:    doc.ID,
            Score: output[0].Score,
        }
    }
    
    return results
}

func (r *Reranker) Close() {
    r.session.Destroy()
}
```

### 2.2 Retrieval Pipeline Integration

**Task 2.2.1: Combine search and reranking**

File: `internal/core/search.go`
```go
package core

import (
    "context"
    
    "github.com/[user]/codex/internal/storage"
    "github.com/[user]/codex/internal/reranking"
    "github.com/[user]/codex/internal/embedding"
)

type SearchEngine struct {
    qdrant   *storage.QdrantStorage
    reranker *reranking.Reranker
    voyage   *embedding.VoyageClient
    openai   *embedding.OpenAIClient
}

type SearchRequest struct {
    Query string
    Types []string
    Scope string
    Limit int
}

type SearchResult struct {
    ID      string
    Type    string
    Title   string
    Content string
    Score   float64
}

func (e *SearchEngine) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
    // 1. Generate query embedding
    queryVec, err := e.voyage.EmbedCodeQuery(ctx, req.Query)
    if err != nil {
        return nil, err
    }
    
    // 2. Hybrid search (50 candidates)
    candidates, err := e.qdrant.HybridSearch(ctx, storage.SearchParams{
        Query:       req.Query,
        QueryVector: queryVec,
        Types:       req.Types,
        Scope:       req.Scope,
        Limit:       50,
        UseHybrid:   true,
    })
    if err != nil {
        return nil, err
    }
    
    // 3. Multi-stage reranking (50 -> 20 -> 10)
    var docs []reranking.Document
    for _, c := range candidates {
        docs = append(docs, reranking.Document{
            ID:      c.ID,
            Content: c.Content,
        })
    }
    
    reranked, err := e.reranker.Rerank(req.Query, docs, req.Limit)
    if err != nil {
        return nil, err
    }
    
    // 4. Build results
    results := make([]SearchResult, len(reranked))
    for i, r := range reranked {
        for _, c := range candidates {
            if c.ID == r.ID {
                results[i] = SearchResult{
                    ID:      c.ID,
                    Type:    c.Type,
                    Title:   c.Title,
                    Content: c.Content,
                    Score:   r.Score,
                }
                break
            }
        }
    }
    
    return results, nil
}
```

### 2.3 Validation Checkpoint

**Acceptance Criteria for Phase 2:**
- [ ] BGE models load via hugot
- [ ] Stage 1 reranking (base) works
- [ ] Stage 2 reranking (v2-m3) works
- [ ] Full pipeline: hybrid search → rerank → results
- [ ] Latency < 500ms for typical queries

---

## Phase 3: AST Chunking & Contextual Retrieval (Week 3)

### Goal
Intelligent chunking for code and contextual enrichment for docs.

### 3.1 Tree-sitter AST Chunking

**Task 3.1.1: AST-aware code chunker**

File: `internal/chunking/ast.go`
```go
package chunking

import (
    "context"
    
    sitter "github.com/smacker/go-tree-sitter"
    "github.com/smacker/go-tree-sitter/golang"
    "github.com/smacker/go-tree-sitter/python"
    "github.com/smacker/go-tree-sitter/typescript"
)

type ASTChunker struct {
    parsers map[string]*sitter.Parser
}

type CodeChunk struct {
    Content    string
    Type       string  // function, class, method, module
    Name       string
    StartLine  int
    EndLine    int
    FilePath   string
    Signature  string  // For functions: "func (r *Receiver) Name(args) returns"
    Imports    []string
}

func NewASTChunker() *ASTChunker {
    parsers := make(map[string]*sitter.Parser)
    
    // Go
    goParser := sitter.NewParser()
    goParser.SetLanguage(golang.GetLanguage())
    parsers["go"] = goParser
    
    // Python
    pyParser := sitter.NewParser()
    pyParser.SetLanguage(python.GetLanguage())
    parsers["py"] = pyParser
    
    // TypeScript
    tsParser := sitter.NewParser()
    tsParser.SetLanguage(typescript.GetLanguage())
    parsers["ts"] = tsParser
    parsers["tsx"] = tsParser
    
    return &ASTChunker{parsers: parsers}
}

func (c *ASTChunker) ChunkFile(content []byte, lang, filePath string) ([]CodeChunk, error) {
    parser, ok := c.parsers[lang]
    if !ok {
        return c.fallbackChunk(content, filePath), nil
    }
    
    tree, err := parser.ParseCtx(context.Background(), nil, content)
    if err != nil {
        return nil, err
    }
    defer tree.Close()
    
    var chunks []CodeChunk
    
    cursor := sitter.NewTreeCursor(tree.RootNode())
    defer cursor.Close()
    
    c.walkTree(cursor, content, filePath, &chunks)
    
    return chunks, nil
}

func (c *ASTChunker) walkTree(cursor *sitter.TreeCursor, content []byte, filePath string, chunks *[]CodeChunk) {
    node := cursor.CurrentNode()
    
    // Extract semantic units
    switch node.Type() {
    case "function_declaration", "method_declaration", "function_definition":
        chunk := c.extractFunction(node, content, filePath)
        *chunks = append(*chunks, chunk)
        
    case "class_declaration", "class_definition":
        chunk := c.extractClass(node, content, filePath)
        *chunks = append(*chunks, chunk)
        
    case "type_declaration":
        chunk := c.extractType(node, content, filePath)
        *chunks = append(*chunks, chunk)
    }
    
    // Recurse into children
    if cursor.GoToFirstChild() {
        c.walkTree(cursor, content, filePath, chunks)
        for cursor.GoToNextSibling() {
            c.walkTree(cursor, content, filePath, chunks)
        }
        cursor.GoToParent()
    }
}

func (c *ASTChunker) extractFunction(node *sitter.Node, content []byte, filePath string) CodeChunk {
    return CodeChunk{
        Content:   string(content[node.StartByte():node.EndByte()]),
        Type:      "function",
        Name:      extractName(node, content),
        StartLine: int(node.StartPoint().Row) + 1,
        EndLine:   int(node.EndPoint().Row) + 1,
        FilePath:  filePath,
        Signature: extractSignature(node, content),
    }
}
```

### 3.2 Contextual Retrieval

**Task 3.2.1: Haiku context generator**

File: `internal/chunking/contextual.go`
```go
package chunking

import (
    "context"
    "fmt"
    
    "github.com/anthropics/anthropic-sdk-go"
)

type ContextualChunker struct {
    client *anthropic.Client
}

type DocChunk struct {
    OriginalContent string
    Context         string  // Generated by Haiku
    EnrichedContent string  // Context + Original
    FilePath        string
    Section         string
}

func NewContextualChunker(apiKey string) *ContextualChunker {
    client := anthropic.NewClient(anthropic.WithAPIKey(apiKey))
    return &ContextualChunker{client: client}
}

func (c *ContextualChunker) EnrichChunk(ctx context.Context, chunk, documentContext string) (string, error) {
    prompt := fmt.Sprintf(`<document>
%s
</document>

Here is a chunk from the document:
<chunk>
%s
</chunk>

Please provide a short, succinct context (1-2 sentences) to situate this chunk within the overall document. Focus on what makes this chunk findable - what queries would someone use to find this information?

Context:`, documentContext, chunk)

    resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
        Model:     anthropic.F(anthropic.ModelClaude3Haiku20240307),
        MaxTokens: anthropic.Int(100),
        Messages: anthropic.F([]anthropic.MessageParam{
            anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
        }),
    })
    if err != nil {
        return "", err
    }
    
    context := resp.Content[0].Text
    
    return context, nil
}

func (c *ContextualChunker) ChunkDocument(ctx context.Context, content, filePath string) ([]DocChunk, error) {
    // Split into sections
    sections := splitMarkdown(content)
    
    var chunks []DocChunk
    
    for _, section := range sections {
        // Get document context (title, TOC, first few paragraphs)
        docContext := extractDocumentContext(content)
        
        // Enrich with Haiku
        context, err := c.EnrichChunk(ctx, section.Content, docContext)
        if err != nil {
            // Fall back to no context on error
            context = ""
        }
        
        chunks = append(chunks, DocChunk{
            OriginalContent: section.Content,
            Context:         context,
            EnrichedContent: context + "\n\n" + section.Content,
            FilePath:        filePath,
            Section:         section.Title,
        })
    }
    
    return chunks, nil
}
```

### 3.3 Validation Checkpoint

**Acceptance Criteria for Phase 3:**
- [ ] Tree-sitter parses Go, Python, TypeScript
- [ ] Functions/classes extracted as coherent chunks
- [ ] Haiku generates useful context for doc chunks
- [ ] Enriched chunks improve retrieval accuracy

---

## Phase 4: Web UI & Polish (Week 4)

### Goal
Web interface for browsing knowledge and admin tasks.

### 4.1 Web Server

**Task 4.1.1: Gin-based web server**

File: `internal/web/server.go`
```go
package web

import (
    "github.com/gin-gonic/gin"
    "github.com/[user]/codex/internal/core"
)

type Server struct {
    engine *core.SearchEngine
    router *gin.Engine
}

func NewServer(engine *core.SearchEngine) *Server {
    router := gin.Default()
    
    s := &Server{
        engine: engine,
        router: router,
    }
    
    // Load templates
    router.LoadHTMLGlob("web/templates/*")
    router.Static("/static", "web/static")
    
    // Routes
    router.GET("/", s.handleIndex)
    router.GET("/search", s.handleSearch)
    router.GET("/item/:id", s.handleItem)
    router.GET("/browse", s.handleBrowse)
    
    // API routes
    api := router.Group("/api")
    {
        api.GET("/search", s.handleAPISearch)
        api.GET("/item/:id", s.handleAPIItem)
        api.POST("/item", s.handleAPICreate)
        api.PUT("/item/:id", s.handleAPIUpdate)
        api.DELETE("/item/:id", s.handleAPIDelete)
    }
    
    return s
}

func (s *Server) Run(addr string) error {
    return s.router.Run(addr)
}
```

**Task 4.1.2: Search handler**

File: `internal/web/handlers.go`
```go
package web

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/[user]/codex/internal/core"
)

func (s *Server) handleSearch(c *gin.Context) {
    query := c.Query("q")
    types := c.QueryArray("type")
    
    results, err := s.engine.Search(c.Request.Context(), core.SearchRequest{
        Query: query,
        Types: types,
        Limit: 20,
    })
    if err != nil {
        c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
        return
    }
    
    c.HTML(http.StatusOK, "search.html", gin.H{
        "query":   query,
        "results": results,
        "count":   len(results),
    })
}

func (s *Server) handleItem(c *gin.Context) {
    id := c.Param("id")
    
    item, err := s.engine.Get(c.Request.Context(), id)
    if err != nil {
        c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Item not found"})
        return
    }
    
    c.HTML(http.StatusOK, "item.html", gin.H{
        "item": item,
    })
}

func (s *Server) handleAPISearch(c *gin.Context) {
    query := c.Query("q")
    types := c.QueryArray("type")
    
    results, err := s.engine.Search(c.Request.Context(), core.SearchRequest{
        Query: query,
        Types: types,
        Limit: 20,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "query":   query,
        "results": results,
        "count":   len(results),
    })
}
```

### 4.2 Update MCP Server

**Task 4.2.1: Update RECALL MCP to use Codex v1**

The MCP server now uses the full Codex search engine instead of SQLite FTS.

### 4.3 Validation Checkpoint

**Acceptance Criteria for Phase 4:**
- [ ] Web UI shows search results
- [ ] Item detail pages work
- [ ] API endpoints functional
- [ ] MCP server uses Codex v1 search

---

## Dependencies Summary

```go
// go.mod additions for Codex v1
require (
    github.com/qdrant/go-client v1.9.0
    github.com/knights-analytics/hugot v0.3.0
    github.com/smacker/go-tree-sitter v0.0.0-20230720070738-0d0a9f78d8f8
    github.com/anthropics/anthropic-sdk-go v0.1.0
    github.com/gin-gonic/gin v1.9.1
)
```

**System Dependencies:**
- ONNX Runtime (`libonnxruntime.so`)
- libtokenizers (via hugot)

**External Services:**
- Voyage AI API (embeddings)
- OpenAI API (embeddings)
- Anthropic API (Haiku for contextual retrieval)

---

## Environment Variables

```bash
# Required
VOYAGE_API_KEY=voy-xxx
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx

# Optional
QDRANT_ADDR=localhost:6334
CODEX_WEB_PORT=8080
CODEX_MODELS_PATH=./models
```
