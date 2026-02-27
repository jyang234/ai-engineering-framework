package chunking

import (
	"fmt"
	"strings"
	"testing"
)

func TestGoFunctionExtraction(t *testing.T) {
	source := `package main

import "fmt"

func Hello(name string) string {
	return fmt.Sprintf("hello %s", name)
}

func Add(a, b int) int {
	return a + b
}
`
	chunker := NewASTChunker()
	defer chunker.Close()

	chunks, err := chunker.ChunkFile([]byte(source), "go", "main.go")
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// First function: Hello
	if chunks[0].Name != "Hello" {
		t.Errorf("chunks[0].Name = %q, want %q", chunks[0].Name, "Hello")
	}
	if chunks[0].Type != "function" {
		t.Errorf("chunks[0].Type = %q, want %q", chunks[0].Type, "function")
	}
	if !strings.Contains(chunks[0].Content, "fmt.Sprintf") {
		t.Error("chunks[0].Content missing function body")
	}
	if chunks[0].Language != "go" {
		t.Errorf("chunks[0].Language = %q, want %q", chunks[0].Language, "go")
	}
	if chunks[0].FilePath != "main.go" {
		t.Errorf("chunks[0].FilePath = %q, want %q", chunks[0].FilePath, "main.go")
	}
	if !strings.Contains(chunks[0].Signature, "func Hello(name string) string") {
		t.Errorf("chunks[0].Signature = %q, want to contain function declaration", chunks[0].Signature)
	}

	// Second function: Add
	if chunks[1].Name != "Add" {
		t.Errorf("chunks[1].Name = %q, want %q", chunks[1].Name, "Add")
	}
	if chunks[1].Type != "function" {
		t.Errorf("chunks[1].Type = %q, want %q", chunks[1].Type, "function")
	}

	// Line numbers (1-indexed)
	if chunks[0].StartLine < 1 || chunks[0].EndLine < chunks[0].StartLine {
		t.Errorf("chunks[0] line range invalid: %d-%d", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[1].StartLine <= chunks[0].EndLine {
		t.Errorf("chunks[1].StartLine (%d) should be after chunks[0].EndLine (%d)",
			chunks[1].StartLine, chunks[0].EndLine)
	}
}

func TestGoMethodAndTypeExtraction(t *testing.T) {
	source := `package main

type Server struct {
	port int
	host string
}

func (s *Server) Start() error {
	return nil
}
`
	chunker := NewASTChunker()
	defer chunker.Close()

	chunks, err := chunker.ChunkFile([]byte(source), "go", "server.go")
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}

	// Should have type + method
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// Find chunks by type
	var typeChunk, methodChunk *CodeChunk
	for i := range chunks {
		switch chunks[i].Type {
		case "type":
			typeChunk = &chunks[i]
		case "method":
			methodChunk = &chunks[i]
		}
	}

	if typeChunk == nil {
		t.Fatal("missing type chunk")
	}
	if typeChunk.Name != "Server" {
		t.Errorf("type chunk name = %q, want %q", typeChunk.Name, "Server")
	}
	if !strings.Contains(typeChunk.Content, "port int") {
		t.Error("type chunk missing struct fields")
	}

	if methodChunk == nil {
		t.Fatal("missing method chunk")
	}
	if methodChunk.Name != "Start" {
		t.Errorf("method chunk name = %q, want %q", methodChunk.Name, "Start")
	}
}

func TestPythonExtraction(t *testing.T) {
	source := `def greet(name):
    return f"hello {name}"

class Calculator:
    def __init__(self):
        self.result = 0

    def add(self, x):
        self.result += x
        return self
`
	chunker := NewASTChunker()
	defer chunker.Close()

	chunks, err := chunker.ChunkFile([]byte(source), "python", "calc.py")
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}

	// Python walks recursively, so we'll get: greet function, Calculator class,
	// and potentially nested functions (__init__, add) inside the class.
	// At minimum we need the top-level function and the class.
	var funcChunk, classChunk *CodeChunk
	for i := range chunks {
		if chunks[i].Type == "function" && chunks[i].Name == "greet" {
			funcChunk = &chunks[i]
		}
		if chunks[i].Type == "class" && chunks[i].Name == "Calculator" {
			classChunk = &chunks[i]
		}
	}

	if funcChunk == nil {
		t.Fatal("missing greet function chunk")
	}
	if funcChunk.Language != "python" {
		t.Errorf("funcChunk.Language = %q, want %q", funcChunk.Language, "python")
	}

	if classChunk == nil {
		t.Fatal("missing Calculator class chunk")
	}
	if !strings.Contains(classChunk.Content, "def __init__") {
		t.Error("class chunk should include method bodies")
	}
}

func TestTypeScriptExtraction(t *testing.T) {
	source := `interface Config {
    port: number;
    host: string;
}

function createServer(config: Config): void {
    console.log(config.port);
}

class App {
    start(): void {}
}
`
	chunker := NewASTChunker()
	defer chunker.Close()

	chunks, err := chunker.ChunkFile([]byte(source), "typescript", "app.ts")
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}

	names := map[string]bool{}
	for _, ch := range chunks {
		if ch.Name != "" {
			names[ch.Name] = true
		}
	}

	// We expect at least: interface (type), function, class
	// Note: method "start" inside class might also be extracted
	if !names["Config"] {
		t.Error("missing Config interface chunk")
	}
	if !names["createServer"] {
		t.Error("missing createServer function chunk")
	}
	if !names["App"] {
		t.Error("missing App class chunk")
	}
}

func TestJavaScriptAliases(t *testing.T) {
	source := `function hello() {
    return "hi";
}
`
	chunker := NewASTChunker()
	defer chunker.Close()

	// All these language aliases should use the TS parser and produce chunks
	languages := []string{"typescript", "tsx", "javascript", "jsx"}
	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			chunks, err := chunker.ChunkFile([]byte(source), lang, "test."+lang)
			if err != nil {
				t.Fatalf("ChunkFile(%s): %v", lang, err)
			}
			if len(chunks) == 0 {
				t.Errorf("expected at least 1 chunk for language %s, got 0", lang)
			}
		})
	}
}

func TestFallbackBehavior(t *testing.T) {
	chunker := NewASTChunker()
	defer chunker.Close()

	t.Run("unsupported_language", func(t *testing.T) {
		rustCode := `fn main() {
    println!("hello");
}
`
		chunks, err := chunker.ChunkFile([]byte(rustCode), "rust", "main.rs")
		if err != nil {
			t.Fatalf("ChunkFile: %v", err)
		}
		if len(chunks) == 0 {
			t.Fatal("expected fallback chunks for unsupported language")
		}
		for _, ch := range chunks {
			if ch.Type != "chunk" {
				t.Errorf("fallback chunk type = %q, want %q", ch.Type, "chunk")
			}
			if ch.Name != "" {
				t.Errorf("fallback chunk name = %q, want empty", ch.Name)
			}
		}
	})

	t.Run("empty_content", func(t *testing.T) {
		chunks, err := chunker.ChunkFile([]byte(""), "go", "empty.go")
		if err != nil {
			t.Fatalf("ChunkFile: %v", err)
		}
		// Empty content: fallback produces no chunks (trimmed is empty)
		if len(chunks) != 0 {
			t.Errorf("expected 0 chunks for empty content, got %d", len(chunks))
		}
	})

	t.Run("only_comments", func(t *testing.T) {
		chunks, err := chunker.ChunkFile([]byte("// just a comment\n"), "go", "comment.go")
		if err != nil {
			t.Fatalf("ChunkFile: %v", err)
		}
		// No AST nodes extracted → falls back to line-based
		for _, ch := range chunks {
			if ch.Type != "chunk" {
				t.Errorf("expected fallback type 'chunk', got %q", ch.Type)
			}
		}
	})
}

func TestFallbackChunkSizing(t *testing.T) {
	// Generate a 250-line file
	var lines []string
	for i := 1; i <= 250; i++ {
		lines = append(lines, fmt.Sprintf("line %d content here", i))
	}
	content := strings.Join(lines, "\n")

	chunker := NewASTChunker()
	defer chunker.Close()

	// Use "rust" to force fallback (unsupported language)
	chunks, err := chunker.ChunkFile([]byte(content), "rust", "big.rs")
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 250 lines, got %d", len(chunks))
	}

	// Chunk 1: lines 1-100
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 100 {
		t.Errorf("chunk[0] range = %d-%d, want 1-100", chunks[0].StartLine, chunks[0].EndLine)
	}
	// Chunk 2: lines 91-190 (100-line window, 10-line overlap)
	if chunks[1].StartLine != 91 || chunks[1].EndLine != 190 {
		t.Errorf("chunk[1] range = %d-%d, want 91-190", chunks[1].StartLine, chunks[1].EndLine)
	}
	// Chunk 3: lines 181-250
	if chunks[2].StartLine != 181 || chunks[2].EndLine != 250 {
		t.Errorf("chunk[2] range = %d-%d, want 181-250", chunks[2].StartLine, chunks[2].EndLine)
	}

	// Verify overlap: last 10 lines of chunk 0 should appear in chunk 1
	chunk0Lines := strings.Split(chunks[0].Content, "\n")
	chunk1Lines := strings.Split(chunks[1].Content, "\n")
	overlapFromChunk0 := chunk0Lines[len(chunk0Lines)-10:]
	overlapFromChunk1 := chunk1Lines[:10]
	for i := 0; i < 10; i++ {
		if overlapFromChunk0[i] != overlapFromChunk1[i] {
			t.Errorf("overlap mismatch at line %d: %q vs %q",
				i, overlapFromChunk0[i], overlapFromChunk1[i])
			break
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filePath string
		want     string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.ts", "typescript"},
		{"component.tsx", "tsx"},
		{"index.js", "javascript"},
		{"App.jsx", "jsx"},
		{"main.rs", "rust"},
		{"Main.java", "java"},
		{"util.c", "c"},
		{"util.h", "c"},
		{"util.cpp", "cpp"},
		{"util.hpp", "cpp"},
		{"Makefile", "unknown"},
		{"README.md", "unknown"},
		{"UPPER.GO", "go"}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := DetectLanguage(tt.filePath)
			if got != tt.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestSupportedLanguagesAndAvailability(t *testing.T) {
	chunker := NewASTChunker()
	defer chunker.Close()

	if !chunker.IsAvailable() {
		t.Error("IsAvailable() = false, want true")
	}

	langs := chunker.GetSupportedLanguages()
	expected := []string{"go", "python", "typescript", "tsx", "javascript", "jsx"}
	if len(langs) != len(expected) {
		t.Fatalf("GetSupportedLanguages() returned %d languages, want %d", len(langs), len(expected))
	}
	for i, want := range expected {
		if langs[i] != want {
			t.Errorf("langs[%d] = %q, want %q", i, langs[i], want)
		}
	}

	if err := chunker.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}
