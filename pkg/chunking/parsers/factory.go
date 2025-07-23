package parsers

import (
	"github.com/developer-mesh/developer-mesh/pkg/chunking"
)

// NewParserFactory creates a new parser factory that registers all supported language parsers
func NewParserFactory() map[chunking.Language]chunking.LanguageParser {
	parsers := make(map[chunking.Language]chunking.LanguageParser)

	// Register Go parser
	goParser := NewGoParser()
	parsers[goParser.GetLanguage()] = goParser

	// Register JavaScript parser
	jsParser := NewJavaScriptParser()
	parsers[jsParser.GetLanguage()] = jsParser

	// Register Python parser
	pyParser := NewPythonParser()
	parsers[pyParser.GetLanguage()] = pyParser

	// Register HCL (Terraform) parser
	hclParser := NewHCLParser()
	parsers[hclParser.GetLanguage()] = hclParser

	// Register TypeScript parser
	tsParser := NewTypeScriptParser()
	parsers[tsParser.GetLanguage()] = tsParser

	// Register Shell parser
	shellParser := NewShellParser()
	parsers[shellParser.GetLanguage()] = shellParser

	// Register Rust parser
	rustParser := NewRustParser()
	parsers[rustParser.GetLanguage()] = rustParser

	// Register Kotlin parser
	kotlinParser := NewKotlinParser()
	parsers[kotlinParser.GetLanguage()] = kotlinParser

	return parsers
}

// InitializeChunkingService creates and initializes a new chunking service with all supported parsers
func InitializeChunkingService() *chunking.ChunkingService {
	service := chunking.NewChunkingService()

	// Get all parsers from the factory
	parsers := NewParserFactory()

	// Register all parsers with the service
	for _, parser := range parsers {
		service.RegisterParser(parser)
	}

	return service
}
