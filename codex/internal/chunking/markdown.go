package chunking

import (
	"regexp"
	"strings"
)

var headerRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// SplitMarkdown splits markdown content into sections based on headers
func SplitMarkdown(content string) []MarkdownSection {
	lines := strings.Split(content, "\n")

	var sections []MarkdownSection
	var currentSection *MarkdownSection
	var currentLines []string

	for i, line := range lines {
		lineNum := i + 1

		// Check if this line is a header
		if matches := headerRegex.FindStringSubmatch(line); matches != nil {
			// Save previous section if exists
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(strings.Join(currentLines, "\n"))
				currentSection.EndLine = lineNum - 1
				if currentSection.Content != "" {
					sections = append(sections, *currentSection)
				}
			}

			// Start new section
			level := len(matches[1])
			title := matches[2]
			currentSection = &MarkdownSection{
				Title:     title,
				Level:     level,
				StartLine: lineNum,
			}
			currentLines = []string{}
		} else if currentSection != nil {
			currentLines = append(currentLines, line)
		} else {
			// Content before first header - create implicit section
			if strings.TrimSpace(line) != "" {
				currentSection = &MarkdownSection{
					Title:     "(Introduction)",
					Level:     0,
					StartLine: 1,
				}
				currentLines = []string{line}
			}
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(strings.Join(currentLines, "\n"))
		currentSection.EndLine = len(lines)
		if currentSection.Content != "" {
			sections = append(sections, *currentSection)
		}
	}

	// If no sections found, treat entire content as one section
	if len(sections) == 0 && strings.TrimSpace(content) != "" {
		sections = append(sections, MarkdownSection{
			Title:     "(Document)",
			Content:   strings.TrimSpace(content),
			Level:     0,
			StartLine: 1,
			EndLine:   len(lines),
		})
	}

	return sections
}

// ChunkMarkdown chunks markdown content with optional size limits
func ChunkMarkdown(content string, maxChunkSize int) []MarkdownSection {
	sections := SplitMarkdown(content)

	if maxChunkSize <= 0 {
		return sections
	}

	// Further split sections that are too large
	var result []MarkdownSection
	for _, section := range sections {
		if len(section.Content) <= maxChunkSize {
			result = append(result, section)
		} else {
			// Split large sections by paragraphs
			chunks := splitByParagraphs(section.Content, maxChunkSize)
			for i, chunk := range chunks {
				result = append(result, MarkdownSection{
					Title:     section.Title,
					Content:   chunk,
					Level:     section.Level,
					StartLine: section.StartLine, // Approximate
					EndLine:   section.EndLine,
				})
				_ = i // suppress unused warning
			}
		}
	}

	return result
}

// splitByParagraphs splits content into chunks at paragraph boundaries
func splitByParagraphs(content string, maxSize int) []string {
	paragraphs := strings.Split(content, "\n\n")

	var chunks []string
	var current strings.Builder

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Check if adding this paragraph would exceed max size
		if current.Len() > 0 && current.Len()+len(para)+2 > maxSize {
			chunks = append(chunks, current.String())
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}
