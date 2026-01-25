package reranking

import (
	"log"
	"os"
	"path/filepath"
	"sort"
)

// Reranker provides multi-stage reranking using BGE models
// TODO: Integrate with Hugot library when available for ONNX inference
type Reranker struct {
	modelsPath string
	available  bool
	// Future: Hugot session and pipelines
	// session *hugot.Session
	// stage1  hugot.FeatureExtractionPipeline
	// stage2  hugot.FeatureExtractionPipeline
}

// NewReranker creates a new reranker instance
func NewReranker(modelsPath string) (*Reranker, error) {
	// Check if models exist
	stage1Path := filepath.Join(modelsPath, "bge-reranker-base", "model.onnx")
	stage2Path := filepath.Join(modelsPath, "bge-reranker-v2-m3", "model.onnx")

	stage1Exists := fileExists(stage1Path)
	stage2Exists := fileExists(stage2Path)

	reranker := &Reranker{
		modelsPath: modelsPath,
		available:  false,
	}

	if !stage1Exists && !stage2Exists {
		log.Printf("INFO: No reranker models found at %s, using fallback scoring", modelsPath)
		return reranker, nil
	}

	// TODO: Initialize Hugot session when library is stable
	// For now, log that models exist but we're using fallback
	log.Printf("INFO: Reranker models found (stage1: %v, stage2: %v) but Hugot integration pending", stage1Exists, stage2Exists)
	log.Printf("INFO: Using fallback score-based reranking")

	// When Hugot is available:
	// session, err := hugot.NewSession()
	// pipeline, err := session.NewPipeline("featureExtraction", stage1Dir)
	// reranker.stage1 = pipeline.(hugot.FeatureExtractionPipeline)
	// reranker.available = true

	return reranker, nil
}

// Rerank performs multi-stage reranking
func (r *Reranker) Rerank(query string, documents []Document, limit int) ([]RerankResult, error) {
	if !r.available {
		// Fallback: return documents in original order with placeholder scores
		results := make([]RerankResult, len(documents))
		for i, doc := range documents {
			results[i] = RerankResult{
				ID:    doc.ID,
				Score: 1.0 - float64(i)*0.01, // Decreasing scores
			}
		}
		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}

	// When Hugot is available, implement two-stage reranking:
	// Stage 1: Fast rerank with BGE-base (50 -> 20)
	stage1Results := r.rerankStage1(query, documents)
	sort.Slice(stage1Results, func(i, j int) bool {
		return stage1Results[i].Score > stage1Results[j].Score
	})

	// Take top 20 for stage 2
	top20 := stage1Results
	if len(top20) > 20 {
		top20 = top20[:20]
	}

	// Get documents for stage 2
	top20Docs := make([]Document, len(top20))
	docMap := make(map[string]Document)
	for _, d := range documents {
		docMap[d.ID] = d
	}
	for i, res := range top20 {
		top20Docs[i] = docMap[res.ID]
	}

	// Stage 2: Precise rerank with BGE-v2-m3 (20 -> limit)
	stage2Results := r.rerankStage2(query, top20Docs)
	sort.Slice(stage2Results, func(i, j int) bool {
		return stage2Results[i].Score > stage2Results[j].Score
	})

	if len(stage2Results) > limit {
		stage2Results = stage2Results[:limit]
	}

	return stage2Results, nil
}

// rerankStage1 uses BGE-reranker-base
func (r *Reranker) rerankStage1(query string, docs []Document) []RerankResult {
	// TODO: Implement with Hugot when available
	// Format inputs as "query [SEP] document" pairs
	// Run through BGE-reranker-base model

	// Fallback: return original order with decreasing scores
	results := make([]RerankResult, len(docs))
	for i, doc := range docs {
		results[i] = RerankResult{
			ID:    doc.ID,
			Score: 1.0 - float64(i)*0.01,
		}
	}
	return results
}

// rerankStage2 uses BGE-reranker-v2-m3
func (r *Reranker) rerankStage2(query string, docs []Document) []RerankResult {
	// TODO: Implement with Hugot when available
	// Format inputs as "query [SEP] document" pairs
	// Run through BGE-reranker-v2-m3 model

	// Fallback: return original order with decreasing scores
	results := make([]RerankResult, len(docs))
	for i, doc := range docs {
		results[i] = RerankResult{
			ID:    doc.ID,
			Score: 1.0 - float64(i)*0.01,
		}
	}
	return results
}

// Close releases resources
func (r *Reranker) Close() {
	// TODO: Close Hugot session when implemented
	// if r.session != nil {
	//     r.session.Destroy()
	// }
}

// IsAvailable returns whether reranking is available
func (r *Reranker) IsAvailable() bool {
	return r.available
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
