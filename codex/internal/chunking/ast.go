package chunking

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// ASTChunker extracts semantic code chunks using Tree-sitter
type ASTChunker struct {
	goParser *sitter.Parser
	pyParser *sitter.Parser
	tsParser *sitter.Parser
	available bool
}

// NewASTChunker creates a new AST-based chunker
func NewASTChunker() *ASTChunker {
	chunker := &ASTChunker{
		available: true,
	}

	// Initialize Go parser
	chunker.goParser = sitter.NewParser()
	chunker.goParser.SetLanguage(golang.GetLanguage())

	// Initialize Python parser
	chunker.pyParser = sitter.NewParser()
	chunker.pyParser.SetLanguage(python.GetLanguage())

	// Initialize TypeScript parser
	chunker.tsParser = sitter.NewParser()
	chunker.tsParser.SetLanguage(typescript.GetLanguage())

	return chunker
}

// ChunkFile extracts semantic chunks from a source file
func (c *ASTChunker) ChunkFile(content []byte, lang, filePath string) ([]CodeChunk, error) {
	if !c.available {
		return c.fallbackChunk(content, filePath, lang), nil
	}

	// Parse based on language
	var tree *sitter.Tree
	switch lang {
	case "go":
		tree = c.goParser.Parse(nil, content)
	case "python":
		tree = c.pyParser.Parse(nil, content)
	case "typescript", "tsx", "javascript", "jsx":
		tree = c.tsParser.Parse(nil, content)
	default:
		// Unsupported language - fall back
		return c.fallbackChunk(content, filePath, lang), nil
	}

	if tree == nil {
		// Parsing failed - fall back
		return c.fallbackChunk(content, filePath, lang), nil
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode == nil {
		return c.fallbackChunk(content, filePath, lang), nil
	}

	// Extract chunks based on language
	var chunks []CodeChunk
	switch lang {
	case "go":
		chunks = c.extractGoChunks(rootNode, content, filePath)
	case "python":
		chunks = c.extractPythonChunks(rootNode, content, filePath)
	case "typescript", "tsx", "javascript", "jsx":
		chunks = c.extractTypeScriptChunks(rootNode, content, filePath)
	}

	// If we didn't extract any chunks, fall back
	if len(chunks) == 0 {
		return c.fallbackChunk(content, filePath, lang), nil
	}

	return chunks, nil
}

// extractGoChunks extracts semantic units from Go code
func (c *ASTChunker) extractGoChunks(node *sitter.Node, content []byte, filePath string) []CodeChunk {
	var chunks []CodeChunk

	// Walk the tree and extract semantic units
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Extract function declarations
		if nodeType == "function_declaration" {
			chunk := c.extractFunctionChunk(n, content, filePath, "go", "function")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Extract method declarations
		if nodeType == "method_declaration" {
			chunk := c.extractFunctionChunk(n, content, filePath, "go", "method")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Extract type declarations (structs, interfaces)
		if nodeType == "type_declaration" {
			chunk := c.extractTypeChunk(n, content, filePath, "go")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return chunks
}

// extractPythonChunks extracts semantic units from Python code
func (c *ASTChunker) extractPythonChunks(node *sitter.Node, content []byte, filePath string) []CodeChunk {
	var chunks []CodeChunk

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Extract function definitions
		if nodeType == "function_definition" {
			chunk := c.extractFunctionChunk(n, content, filePath, "python", "function")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Extract class definitions
		if nodeType == "class_definition" {
			chunk := c.extractClassChunk(n, content, filePath, "python")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return chunks
}

// extractTypeScriptChunks extracts semantic units from TypeScript/JavaScript code
func (c *ASTChunker) extractTypeScriptChunks(node *sitter.Node, content []byte, filePath string) []CodeChunk {
	var chunks []CodeChunk

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Extract function declarations
		if nodeType == "function_declaration" || nodeType == "function" {
			chunk := c.extractFunctionChunk(n, content, filePath, "typescript", "function")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Extract method definitions
		if nodeType == "method_definition" {
			chunk := c.extractFunctionChunk(n, content, filePath, "typescript", "method")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Extract class declarations
		if nodeType == "class_declaration" {
			chunk := c.extractClassChunk(n, content, filePath, "typescript")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Extract interface declarations
		if nodeType == "interface_declaration" {
			chunk := c.extractTypeChunk(n, content, filePath, "typescript")
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return chunks
}

// extractFunctionChunk extracts a function or method chunk
func (c *ASTChunker) extractFunctionChunk(node *sitter.Node, content []byte, filePath, lang, chunkType string) *CodeChunk {
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	nodeContent := content[node.StartByte():node.EndByte()]

	// Extract function name
	name := c.extractName(node, content)

	// Extract signature (first line or until opening brace)
	signature := c.extractSignature(nodeContent)

	return &CodeChunk{
		Content:   string(nodeContent),
		Type:      chunkType,
		Name:      name,
		StartLine: startLine,
		EndLine:   endLine,
		FilePath:  filePath,
		Signature: signature,
		Language:  lang,
	}
}

// extractClassChunk extracts a class chunk
func (c *ASTChunker) extractClassChunk(node *sitter.Node, content []byte, filePath, lang string) *CodeChunk {
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	nodeContent := content[node.StartByte():node.EndByte()]
	name := c.extractName(node, content)
	signature := c.extractSignature(nodeContent)

	return &CodeChunk{
		Content:   string(nodeContent),
		Type:      "class",
		Name:      name,
		StartLine: startLine,
		EndLine:   endLine,
		FilePath:  filePath,
		Signature: signature,
		Language:  lang,
	}
}

// extractTypeChunk extracts a type/interface chunk
func (c *ASTChunker) extractTypeChunk(node *sitter.Node, content []byte, filePath, lang string) *CodeChunk {
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	nodeContent := content[node.StartByte():node.EndByte()]
	name := c.extractName(node, content)
	signature := c.extractSignature(nodeContent)

	return &CodeChunk{
		Content:   string(nodeContent),
		Type:      "type",
		Name:      name,
		StartLine: startLine,
		EndLine:   endLine,
		FilePath:  filePath,
		Signature: signature,
		Language:  lang,
	}
}

// extractName extracts the name from a node
func (c *ASTChunker) extractName(node *sitter.Node, content []byte) string {
	// Look for identifier or name child node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "name" {
			return string(content[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// extractSignature extracts the signature (first line or declaration line)
func (c *ASTChunker) extractSignature(content []byte) string {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return ""
	}

	// For single-line declarations, return the whole thing
	if len(lines) == 1 {
		return strings.TrimSpace(lines[0])
	}

	// For multi-line, find the declaration (up to opening brace or colon)
	signature := ""
	for _, line := range lines {
		signature += line + "\n"
		// Stop at opening brace or colon (common for function/class declarations)
		if strings.Contains(line, "{") || strings.HasSuffix(strings.TrimSpace(line), ":") {
			break
		}
	}

	return strings.TrimSpace(signature)
}

// fallbackChunk provides simple line-based chunking when AST parsing is unavailable
func (c *ASTChunker) fallbackChunk(content []byte, filePath, lang string) []CodeChunk {
	lines := strings.Split(string(content), "\n")

	// Chunk by ~100 lines with some overlap
	const chunkSize = 100
	const overlap = 10

	var chunks []CodeChunk

	for i := 0; i < len(lines); i += chunkSize - overlap {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}

		chunkLines := lines[i:end]
		chunkContent := strings.Join(chunkLines, "\n")

		if strings.TrimSpace(chunkContent) == "" {
			continue
		}

		chunks = append(chunks, CodeChunk{
			Content:   chunkContent,
			Type:      "chunk",
			Name:      "", // No name for fallback chunks
			StartLine: i + 1,
			EndLine:   end,
			FilePath:  filePath,
			Language:  lang,
		})

		if end >= len(lines) {
			break
		}
	}

	return chunks
}

// DetectLanguage detects the programming language from file extension
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filePath)

	switch {
	case strings.HasSuffix(ext, ".go"):
		return "go"
	case strings.HasSuffix(ext, ".py"):
		return "python"
	case strings.HasSuffix(ext, ".ts"):
		return "typescript"
	case strings.HasSuffix(ext, ".tsx"):
		return "tsx"
	case strings.HasSuffix(ext, ".js"):
		return "javascript"
	case strings.HasSuffix(ext, ".jsx"):
		return "jsx"
	case strings.HasSuffix(ext, ".rs"):
		return "rust"
	case strings.HasSuffix(ext, ".java"):
		return "java"
	case strings.HasSuffix(ext, ".c"), strings.HasSuffix(ext, ".h"):
		return "c"
	case strings.HasSuffix(ext, ".cpp"), strings.HasSuffix(ext, ".hpp"):
		return "cpp"
	default:
		return "unknown"
	}
}

// IsAvailable returns whether AST chunking is available
func (c *ASTChunker) IsAvailable() bool {
	return c.available
}

// Close cleans up parser resources
func (c *ASTChunker) Close() error {
	// Tree-sitter parsers don't require explicit cleanup in go-tree-sitter
	return nil
}

// GetSupportedLanguages returns the list of languages supported by AST chunking
func (c *ASTChunker) GetSupportedLanguages() []string {
	return []string{"go", "python", "typescript", "tsx", "javascript", "jsx"}
}

// validateChunk checks if a chunk is valid
func validateChunk(chunk *CodeChunk) error {
	if chunk.Content == "" {
		return fmt.Errorf("chunk content is empty")
	}
	if chunk.FilePath == "" {
		return fmt.Errorf("chunk file path is empty")
	}
	if chunk.StartLine < 1 {
		return fmt.Errorf("invalid start line: %d", chunk.StartLine)
	}
	if chunk.EndLine < chunk.StartLine {
		return fmt.Errorf("end line %d is before start line %d", chunk.EndLine, chunk.StartLine)
	}
	return nil
}
