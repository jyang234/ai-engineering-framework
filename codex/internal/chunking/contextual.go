package chunking

import (
	"context"
	"fmt"
	"strings"
)

// ContextualChunker enriches document chunks with contextual descriptions
// TODO: Implement Anthropic SDK integration for Claude Haiku
type ContextualChunker struct {
	apiKey string
	// client will be *anthropic.Client when SDK integration is complete
}

// NewContextualChunker creates a new contextual chunker
func NewContextualChunker(apiKey string) (*ContextualChunker, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	return &ContextualChunker{
		apiKey: apiKey,
	}, nil
}

// EnrichChunk generates a contextual description for a chunk using Claude 3 Haiku
func (c *ContextualChunker) EnrichChunk(ctx context.Context, chunk, documentContext string) (string, error) {
	// TODO: Implement with Anthropic SDK
	// Prompt template:
	// <document>{documentContext}</document>
	// Here is a chunk from the document:
	// <chunk>{chunk}</chunk>
	// Please provide a short, succinct context (1-2 sentences) to situate this chunk
	// within the overall document. Focus on what makes this chunk findable.
	//
	// Use Claude 3 Haiku (claude-3-haiku-20240307) with max_tokens=100
	// Include retry logic with exponential backoff for rate limits

	return "", fmt.Errorf("contextual enrichment not implemented")
}

// ChunkDocument chunks a document and enriches each chunk
func (c *ContextualChunker) ChunkDocument(ctx context.Context, content, filePath string) ([]DocChunk, error) {
	// Split into sections
	sections := SplitMarkdown(content)

	// Get document context (title, TOC, first few paragraphs)
	docContext := extractDocumentContext(content)

	var chunks []DocChunk

	for _, section := range sections {
		// Enrich with Haiku (when implemented)
		contextStr, err := c.EnrichChunk(ctx, section.Content, docContext)
		if err != nil {
			// Fall back to no context on error
			contextStr = ""
		}

		enriched := section.Content
		if contextStr != "" {
			enriched = contextStr + "\n\n" + section.Content
		}

		chunks = append(chunks, DocChunk{
			OriginalContent: section.Content,
			Context:         contextStr,
			EnrichedContent: enriched,
			FilePath:        filePath,
			Section:         section.Title,
			StartLine:       section.StartLine,
			EndLine:         section.EndLine,
		})
	}

	return chunks, nil
}

// extractDocumentContext extracts context from the beginning of a document
func extractDocumentContext(content string) string {
	lines := strings.Split(content, "\n")

	// Take first ~50 lines or until we've seen the main structure
	var contextLines []string
	for i, line := range lines {
		if i >= 50 {
			break
		}
		contextLines = append(contextLines, line)
	}

	return strings.Join(contextLines, "\n")
}
