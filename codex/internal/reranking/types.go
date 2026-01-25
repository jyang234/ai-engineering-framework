package reranking

// Document represents a document to be reranked
type Document struct {
	ID      string
	Content string
}

// RerankResult represents a reranked document with score
type RerankResult struct {
	ID    string
	Score float64
}
