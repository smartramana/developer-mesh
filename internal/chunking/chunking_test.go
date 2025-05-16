package chunking

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkingService_DetectLanguage(t *testing.T) {
	// Create chunking service
	service := NewChunkingService()

	// Test language detection by file extension
	tests := []struct {
		filename string
		content  string
		expected Language
	}{
		{"test.go", "package main", LanguageGo},
		{"test.js", "function test() {}", LanguageJavaScript},
		{"test.ts", "interface Test {}", LanguageTypeScript},
		{"test.py", "def test():", LanguagePython},
		{"test.java", "public class Test {}", LanguageJava},
		{"test.rb", "def test; end", LanguageRuby},
		{"test.rs", "fn main() {}", LanguageRust},
		{"test.cs", "class Test {}", LanguageCSharp},
		{"test.cpp", "int main() {}", LanguageCPP},
		{"test.c", "int main() {}", LanguageC},
		{"test.kt", "class Test {}", LanguageKotlin},
		{"test.kts", "fun main() {}", LanguageKotlin},
		{"test.unknown", "unknown language", LanguageUnknown},
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			result := service.DetectLanguage(test.filename, test.content)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestChunkingService_FallbackChunking(t *testing.T) {
	// Create chunking service
	service := NewChunkingService()

	// Create sample code content
	code := `First line
Second line
Third line`

	filename := "test.txt"

	// Test fallback chunking
	chunks := service.fallbackChunking(code, filename, LanguageUnknown)

	// Verify the chunk properties
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, ChunkTypeFile, chunks[0].Type)
	assert.Equal(t, filename, chunks[0].Name)
	assert.Equal(t, filename, chunks[0].Path)
	assert.Equal(t, code, chunks[0].Content)
	assert.Equal(t, LanguageUnknown, chunks[0].Language)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, 3, chunks[0].EndLine)
	assert.Equal(t, "fallback", chunks[0].Metadata["chunking_method"])
}

func TestChunkingService_ChunkCode_Unsupported(t *testing.T) {
	// Create a chunking service with no registered parsers
	service := NewChunkingService()

	// Create sample code content for an unsupported language
	code := `This is some sample text
in an unsupported language.`

	filename := "test.txt"

	// Test chunking for unsupported language
	chunks, err := service.ChunkCode(context.Background(), code, filename)

	// Verify error is nil and fallback chunking is used
	assert.NoError(t, err)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, ChunkTypeFile, chunks[0].Type)
	assert.Equal(t, filename, chunks[0].Name)
	assert.Equal(t, filename, chunks[0].Path)
	assert.Equal(t, code, chunks[0].Content)
	assert.Equal(t, LanguageUnknown, chunks[0].Language)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, 2, chunks[0].EndLine)
}

func TestHelperFunctions(t *testing.T) {
	// Test countLines
	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},
		{"single line", 1},
		{"first line\nsecond line", 2},
		{"first\nsecond\nthird", 3},
		{"line\nwith\nempty\nlines\n\nbetween", 6},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := countLines(test.input)
			assert.Equal(t, test.expected, result)
		})
	}

	// Test getFileExtension
	extTests := []struct {
		filename string
		expected string
	}{
		{"file.go", ".go"},
		{"path/to/file.js", ".js"},
		{"file_without_extension", ""},
		{"multiple.dots.in.filename.py", ".py"},
		{"/absolute/path/to/file.cpp", ".cpp"},
	}

	for _, test := range extTests {
		t.Run(test.filename, func(t *testing.T) {
			result := getFileExtension(test.filename)
			assert.Equal(t, test.expected, result)
		})
	}

	// Test generateChunkID
	chunk := &CodeChunk{
		Type:      ChunkTypeFunction,
		Name:      "testFunc",
		Path:      "test.go",
		StartLine: 10,
		EndLine:   20,
	}

	// Ensure the function returns a non-empty string
	id := generateChunkID(chunk.Name, chunk.StartLine, chunk.EndLine)
	assert.NotEmpty(t, id)
}
