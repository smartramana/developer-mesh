package chunking

import (
	"context"
	"io"
	"strconv"
)

// ChunkType represents the type of code chunk
type ChunkType string

const (
	// Different types of code chunks
	ChunkTypeFunction ChunkType = "function"
	ChunkTypeMethod   ChunkType = "method"
	ChunkTypeClass    ChunkType = "class"
	ChunkTypeStruct   ChunkType = "struct"
	ChunkTypeInterface ChunkType = "interface"
	ChunkTypeBlock    ChunkType = "block"
	ChunkTypeFile     ChunkType = "file"
	ChunkTypeComment  ChunkType = "comment"
	ChunkTypeImport   ChunkType = "import"
)

// Language represents the programming language of the code
type Language string

const (
	// Supported languages
	LanguageGo         Language = "go"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguagePython     Language = "python"
	LanguageJava       Language = "java"
	LanguageRuby       Language = "ruby"
	LanguageRust       Language = "rust"
	LanguageCSharp     Language = "csharp"
	LanguageCPP        Language = "cpp"
	LanguageC          Language = "c"
	LanguageUnknown    Language = "unknown"
)

// CodeChunk represents a chunk of code with its metadata
type CodeChunk struct {
	// Unique identifier for the chunk
	ID string `json:"id"`
	
	// Type of the chunk (function, class, etc.)
	Type ChunkType `json:"type"`
	
	// Name of the chunk (function name, class name, etc.)
	Name string `json:"name"`
	
	// Full path to the chunk (e.g., package.class.method)
	Path string `json:"path"`
	
	// The source code of the chunk
	Content string `json:"content"`
	
	// Programming language of the chunk
	Language Language `json:"language"`
	
	// Start line number (1-based)
	StartLine int `json:"start_line"`
	
	// End line number (1-based)
	EndLine int `json:"end_line"`
	
	// Parent chunk ID, if any
	ParentID string `json:"parent_id,omitempty"`
	
	// IDs of chunks that this chunk depends on
	Dependencies []string `json:"dependencies,omitempty"`
	
	// Additional metadata specific to the chunk
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// LanguageParser is the interface for language-specific code parsers
type LanguageParser interface {
	// Parse parses code in the language and returns chunks
	Parse(ctx context.Context, code string, filename string) ([]*CodeChunk, error)
	
	// GetLanguage returns the language this parser handles
	GetLanguage() Language
}

// ChunkingService provides code chunking capabilities
type ChunkingService struct {
	parsers map[Language]LanguageParser
}

// NewChunkingService creates a new ChunkingService
func NewChunkingService() *ChunkingService {
	return &ChunkingService{
		parsers: make(map[Language]LanguageParser),
	}
}

// RegisterParser registers a language parser
func (s *ChunkingService) RegisterParser(parser LanguageParser) {
	s.parsers[parser.GetLanguage()] = parser
}

// DetectLanguage attempts to detect the language of code based on filename and content
func (s *ChunkingService) DetectLanguage(filename string, content string) Language {
	// First try to detect based on file extension
	extension := getFileExtension(filename)
	
	switch extension {
	case ".go":
		return LanguageGo
	case ".js":
		return LanguageJavaScript
	case ".ts":
		return LanguageTypeScript
	case ".py":
		return LanguagePython
	case ".java":
		return LanguageJava
	case ".rb":
		return LanguageRuby
	case ".rs":
		return LanguageRust
	case ".cs":
		return LanguageCSharp
	case ".cpp", ".cxx", ".cc":
		return LanguageCPP
	case ".c":
		return LanguageC
	}
	
	// TODO: Add more sophisticated language detection based on content
	// For now, return unknown if extension doesn't match
	return LanguageUnknown
}

// ChunkCode chunks the provided code into logical units
func (s *ChunkingService) ChunkCode(ctx context.Context, code string, filename string) ([]*CodeChunk, error) {
	// Detect language
	language := s.DetectLanguage(filename, code)
	
	// Get appropriate parser
	parser, exists := s.parsers[language]
	if !exists {
		// Use a fallback chunking method (e.g., line-based or block-based)
		return s.fallbackChunking(code, filename, language), nil
	}
	
	// Use the language-specific parser
	return parser.Parse(ctx, code, filename)
}

// ChunkReader chunks code from a reader
func (s *ChunkingService) ChunkReader(ctx context.Context, reader io.Reader, filename string) ([]*CodeChunk, error) {
	// Read all content
	code, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	
	return s.ChunkCode(ctx, string(code), filename)
}

// fallbackChunking provides a basic chunking strategy for unsupported languages
func (s *ChunkingService) fallbackChunking(code string, filename string, language Language) []*CodeChunk {
	// Create a single chunk for the entire file
	chunk := &CodeChunk{
		ID:       generateChunkID(filename, 1, len(code)),
		Type:     ChunkTypeFile,
		Name:     filename,
		Path:     filename,
		Content:  code,
		Language: language,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata: map[string]interface{}{
			"chunking_method": "fallback",
		},
	}
	
	return []*CodeChunk{chunk}
}

// Helper functions
func getFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
		if filename[i] == '/' || filename[i] == '\\' {
			break
		}
	}
	return ""
}

func countLines(s string) int {
	count := 1
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}

func generateChunkID(filename string, startLine int, endLine int) string {
	return filename + "#L" + strconv.Itoa(startLine) + "-L" + strconv.Itoa(endLine)
}
