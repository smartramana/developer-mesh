package parsers

import (
	"github.com/S-Corkum/devops-mcp/internal/chunking"
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
	
	// Add more parsers here as they are implemented
	
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
