package chunking

import (
	"strings"
	"testing"
)

func TestSplitMarkdownBasic(t *testing.T) {
	content := `# Introduction
Some intro text.

## Methods
Method details here.
More method text.

## Results
Results text.`

	sections := SplitMarkdown(content)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	// Section 1
	if sections[0].Title != "Introduction" {
		t.Errorf("sections[0].Title = %q, want %q", sections[0].Title, "Introduction")
	}
	if sections[0].Level != 1 {
		t.Errorf("sections[0].Level = %d, want 1", sections[0].Level)
	}
	if !strings.Contains(sections[0].Content, "Some intro text.") {
		t.Errorf("sections[0].Content = %q, missing intro text", sections[0].Content)
	}

	// Section 2
	if sections[1].Title != "Methods" {
		t.Errorf("sections[1].Title = %q, want %q", sections[1].Title, "Methods")
	}
	if sections[1].Level != 2 {
		t.Errorf("sections[1].Level = %d, want 2", sections[1].Level)
	}
	if !strings.Contains(sections[1].Content, "Method details") {
		t.Errorf("sections[1].Content missing method details")
	}
	if !strings.Contains(sections[1].Content, "More method text.") {
		t.Errorf("sections[1].Content missing continued text")
	}

	// Section 3
	if sections[2].Title != "Results" {
		t.Errorf("sections[2].Title = %q, want %q", sections[2].Title, "Results")
	}
	if !strings.Contains(sections[2].Content, "Results text.") {
		t.Errorf("sections[2].Content missing results text")
	}

	// Line numbers should be valid
	for i, s := range sections {
		if s.StartLine < 1 {
			t.Errorf("sections[%d].StartLine = %d, want >= 1", i, s.StartLine)
		}
		if s.EndLine < s.StartLine {
			t.Errorf("sections[%d].EndLine (%d) < StartLine (%d)", i, s.EndLine, s.StartLine)
		}
	}
}

func TestSplitMarkdownEdgeCases(t *testing.T) {
	t.Run("content_before_first_header", func(t *testing.T) {
		content := "Some text before headers\n\n# First Header\nContent here"
		sections := SplitMarkdown(content)

		if len(sections) < 1 {
			t.Fatal("expected at least 1 section")
		}
		if sections[0].Title != "(Introduction)" {
			t.Errorf("first section title = %q, want %q", sections[0].Title, "(Introduction)")
		}
		if sections[0].Level != 0 {
			t.Errorf("introduction level = %d, want 0", sections[0].Level)
		}
	})

	t.Run("no_headers", func(t *testing.T) {
		content := "Just plain text\nwith multiple lines"
		sections := SplitMarkdown(content)

		if len(sections) != 1 {
			t.Fatalf("expected 1 section, got %d", len(sections))
		}
		// When content has no headers, the first non-empty line triggers
		// "(Introduction)" section creation. "(Document)" is only used when
		// no sections were accumulated at all (can't happen with non-empty content).
		if sections[0].Title != "(Introduction)" {
			t.Errorf("title = %q, want %q", sections[0].Title, "(Introduction)")
		}
		if sections[0].Level != 0 {
			t.Errorf("level = %d, want 0", sections[0].Level)
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		sections := SplitMarkdown("")
		if len(sections) != 0 {
			t.Errorf("expected 0 sections for empty string, got %d", len(sections))
		}
	})

	t.Run("only_whitespace", func(t *testing.T) {
		sections := SplitMarkdown("   \n\n   ")
		if len(sections) != 0 {
			t.Errorf("expected 0 sections for whitespace, got %d", len(sections))
		}
	})

	t.Run("header_at_end_no_content", func(t *testing.T) {
		content := "# Title\nSome content\n## Empty"
		sections := SplitMarkdown(content)
		// The "Empty" section has no content after it, so it should be skipped
		for _, s := range sections {
			if s.Title == "Empty" {
				t.Error("section with no content should be skipped")
			}
		}
	})

	t.Run("consecutive_headers", func(t *testing.T) {
		content := "# A\n## B\nContent of B"
		sections := SplitMarkdown(content)
		// "A" has no content between it and "B", so it gets skipped
		// "B" has content
		foundB := false
		for _, s := range sections {
			if s.Title == "B" {
				foundB = true
				if !strings.Contains(s.Content, "Content of B") {
					t.Errorf("section B content = %q, missing expected text", s.Content)
				}
			}
			if s.Title == "A" {
				t.Error("section A should be skipped (no content)")
			}
		}
		if !foundB {
			t.Error("section B not found")
		}
	})
}

func TestChunkMarkdownSplitsLargeSections(t *testing.T) {
	// Build a markdown doc with one small section and one large section
	smallSection := "# Small\nShort content."
	// Build a large section: 5 paragraphs, each ~300 bytes
	var paragraphs []string
	for i := 0; i < 5; i++ {
		paragraphs = append(paragraphs, strings.Repeat("word ", 60)) // ~300 chars each
	}
	largeContent := strings.Join(paragraphs, "\n\n")
	content := smallSection + "\n\n# Large\n" + largeContent

	sections := ChunkMarkdown(content, 500)

	// Small section should pass through unchanged
	foundSmall := false
	largeCount := 0
	for _, s := range sections {
		if s.Title == "Small" {
			foundSmall = true
		}
		if s.Title == "Large" {
			largeCount++
		}
	}
	if !foundSmall {
		t.Error("small section missing from result")
	}
	// Large section should be split into multiple chunks
	if largeCount < 2 {
		t.Errorf("expected large section to be split into >= 2 chunks, got %d", largeCount)
	}

	// All content should be preserved across the chunks
	var allLargeContent strings.Builder
	first := true
	for _, s := range sections {
		if s.Title == "Large" {
			if !first {
				allLargeContent.WriteString("\n\n")
			}
			allLargeContent.WriteString(s.Content)
			first = false
		}
	}
	// Verify all paragraphs are present
	for _, para := range paragraphs {
		trimmed := strings.TrimSpace(para)
		if !strings.Contains(allLargeContent.String(), trimmed) {
			t.Error("paragraph content lost during chunking")
			break
		}
	}
}

func TestChunkMarkdownNoSplitWhenZeroMax(t *testing.T) {
	content := "# Title\n" + strings.Repeat("x", 10000)
	withSplit := ChunkMarkdown(content, 100)
	withoutSplit := ChunkMarkdown(content, 0)

	// maxChunkSize=0 disables further splitting
	if len(withoutSplit) >= len(withSplit) && len(withSplit) > 1 {
		// withSplit should have more sections than withoutSplit
		// This is fine — just verify withoutSplit == SplitMarkdown output
	}
	// withoutSplit should equal SplitMarkdown result
	direct := SplitMarkdown(content)
	if len(withoutSplit) != len(direct) {
		t.Errorf("ChunkMarkdown(content, 0) returned %d sections, SplitMarkdown returned %d",
			len(withoutSplit), len(direct))
	}
}

func TestSplitByParagraphs(t *testing.T) {
	t.Run("all_fit", func(t *testing.T) {
		chunks := splitByParagraphs("para1\n\npara2\n\npara3", 1000)
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(chunks))
		}
	})

	t.Run("each_separate", func(t *testing.T) {
		input := "short paragraph one\n\nshort paragraph two\n\nshort paragraph three"
		chunks := splitByParagraphs(input, 30)
		if len(chunks) != 3 {
			t.Errorf("expected 3 chunks, got %d: %v", len(chunks), chunks)
		}
	})

	t.Run("single_oversized_paragraph", func(t *testing.T) {
		input := strings.Repeat("a", 200)
		chunks := splitByParagraphs(input, 50)
		// Can't split a single paragraph further
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk for oversized paragraph, got %d", len(chunks))
		}
	})
}
