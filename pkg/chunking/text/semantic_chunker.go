package text

import (
	"context"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/chunking"
	"github.com/developer-mesh/developer-mesh/pkg/tokenizer"
)

// SemanticChunker implements semantic-aware text chunking
type SemanticChunker struct {
	tokenizer        tokenizer.Tokenizer
	sentenceSplitter SentenceSplitter
	config           *Config
}

// Config configures the semantic chunker
type Config struct {
	MinChunkSize        int     // Minimum tokens per chunk
	MaxChunkSize        int     // Maximum tokens per chunk
	TargetChunkSize     int     // Target size for chunks
	OverlapSize         int     // Token overlap between chunks
	SimilarityThreshold float32 // For semantic boundaries
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		MinChunkSize:        100,
		MaxChunkSize:        1024,
		TargetChunkSize:     512,
		OverlapSize:         50,
		SimilarityThreshold: 0.5,
	}
}

// NewSemanticChunker creates a new semantic chunker
func NewSemanticChunker(tokenizer tokenizer.Tokenizer, config *Config) *SemanticChunker {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate and set defaults
	if config.TargetChunkSize == 0 {
		config.TargetChunkSize = 512
	}
	if config.MinChunkSize == 0 {
		config.MinChunkSize = 100
	}
	if config.MaxChunkSize == 0 {
		config.MaxChunkSize = 1024
	}

	return &SemanticChunker{
		tokenizer:        tokenizer,
		sentenceSplitter: NewSentenceSplitter(),
		config:           config,
	}
}

// Chunk splits text into semantic chunks
func (s *SemanticChunker) Chunk(ctx context.Context, text string, metadata map[string]interface{}) ([]*chunking.TextChunk, error) {
	// First split by paragraphs (double newline)
	paragraphs := strings.Split(text, "\n\n")

	// Then split each paragraph into sentences
	var allSentences []string
	paragraphBoundaries := make(map[int]bool)
	sentenceIndex := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraSentences := s.sentenceSplitter.Split(para)
		for i, sent := range paraSentences {
			allSentences = append(allSentences, sent)
			// Mark the last sentence of each paragraph as a boundary
			if i == len(paraSentences)-1 {
				paragraphBoundaries[sentenceIndex] = true
			}
			sentenceIndex++
		}
	}

	sentences := allSentences
	if len(sentences) == 0 {
		return []*chunking.TextChunk{}, nil
	}

	chunks := []*chunking.TextChunk{}
	currentChunk := &chunking.TextChunk{
		Metadata:  copyMetadata(metadata),
		StartChar: 0,
	}
	currentTokens := 0
	currentChar := 0

	for i, sentence := range sentences {
		sentenceTokens := s.tokenizer.CountTokens(sentence)

		// Debug: Check paragraph boundaries
		_ = paragraphBoundaries // use the variable

		// Check if single sentence exceeds max size (force split)
		if sentenceTokens > s.config.MaxChunkSize {
			// Handle very long sentences by splitting them
			words := strings.Fields(sentence)
			currentWords := strings.Fields(currentChunk.Content)

			for _, word := range words {
				testContent := strings.Join(append(currentWords, word), " ")
				testTokens := s.tokenizer.CountTokens(testContent)

				if testTokens > s.config.MaxChunkSize && len(currentWords) > 0 {
					// Finalize current chunk
					currentChunk.Content = strings.Join(currentWords, " ")
					currentChunk.EndChar = currentChar
					currentChunk.TokenCount = s.tokenizer.CountTokens(currentChunk.Content)
					chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))

					// Start new chunk with overlap
					overlapText := s.getOverlapText(currentChunk.Content, s.config.OverlapSize)
					currentWords = []string{}
					currentChunk = &chunking.TextChunk{
						Content:   "",
						Metadata:  copyMetadata(metadata),
						StartChar: currentChar,
					}
					// currentTokens will be recalculated below

					// Add overlap if it fits
					if overlapText != "" {
						overlapTokens := s.tokenizer.CountTokens(overlapText)
						wordTokens := s.tokenizer.CountTokens(word)
						if overlapTokens+wordTokens <= s.config.MaxChunkSize {
							currentWords = strings.Fields(overlapText)
							currentChunk.Content = overlapText
							// currentTokens will be recalculated below
						}
					}

					// Add the current word
					currentWords = append(currentWords, word)
					currentChunk.Content = strings.Join(currentWords, " ")
					currentTokens = s.tokenizer.CountTokens(currentChunk.Content)
				} else {
					currentWords = append(currentWords, word)
					currentTokens = testTokens
				}
				currentChar += len(word) + 1 // word + space
			}

			currentChunk.Content = strings.Join(currentWords, " ")
			currentChar-- // Remove last space
			continue
		}

		// Check if adding sentence exceeds max size
		if currentTokens > 0 && currentTokens+sentenceTokens > s.config.MaxChunkSize {
			// Finalize current chunk
			currentChunk.EndChar = currentChar
			chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))

			// Start new chunk with overlap (but don't include it in the chunk immediately)
			overlapText := s.getOverlapText(currentChunk.Content, s.config.OverlapSize)
			currentChunk = &chunking.TextChunk{
				Content:   "",
				Metadata:  copyMetadata(metadata),
				StartChar: currentChar,
			}
			currentTokens = 0

			// Only add overlap if it doesn't make the new chunk exceed max size
			if overlapText != "" {
				overlapTokens := s.tokenizer.CountTokens(overlapText)
				if overlapTokens+sentenceTokens <= s.config.MaxChunkSize {
					currentChunk.Content = overlapText
					currentChunk.StartChar = currentChar - len(overlapText)
					currentTokens = overlapTokens
				}
			}
		}

		// Check if adding this sentence would exceed max size
		potentialTokens := currentTokens + sentenceTokens
		if currentTokens > 0 { // Account for space between sentences
			potentialTokens++
		}

		// Debug logging
		// if i < 5 || i > len(sentences)-5 {
		//     fmt.Printf("[Sentence %d] current=%d, sentence=%d, potential=%d, max=%d\n",
		//         i, currentTokens, sentenceTokens, potentialTokens, s.config.MaxChunkSize)
		// }

		// If adding this sentence would exceed max size, finalize current chunk first
		if potentialTokens > s.config.MaxChunkSize && currentTokens >= s.config.MinChunkSize {
			currentChunk.EndChar = currentChar
			chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))

			// Start new chunk with overlap
			overlapText := s.getOverlapText(currentChunk.Content, s.config.OverlapSize)
			currentChunk = &chunking.TextChunk{
				Content:   "",
				Metadata:  copyMetadata(metadata),
				StartChar: currentChar,
			}
			currentTokens = 0

			// Only add overlap if it doesn't make the new chunk exceed max size with the sentence
			if overlapText != "" {
				overlapTokens := s.tokenizer.CountTokens(overlapText)
				if overlapTokens+sentenceTokens <= s.config.MaxChunkSize {
					currentChunk.Content = overlapText
					currentChunk.StartChar = currentChar - len(overlapText)
					currentTokens = overlapTokens
				}
			}
		}

		// Now add the sentence to current chunk
		if currentChunk.Content != "" {
			currentChunk.Content += " "
			currentChar++   // for the space
			currentTokens++ // for the space token
		}
		currentChunk.Content += sentence
		currentTokens += sentenceTokens
		currentChar += len(sentence)

		// Check if we should create a chunk (target size reached or paragraph boundary)
		shouldSplit := false

		// Force split at paragraph boundaries if we have enough content
		if paragraphBoundaries[i] && currentTokens >= s.config.MinChunkSize {
			shouldSplit = true
		}

		// Also split if we've reached target size and found a semantic boundary
		if currentTokens >= s.config.TargetChunkSize && s.isSemanticBoundary(sentences, i) {
			shouldSplit = true
		}

		// Always split if next sentence would exceed max size
		// Check what the size would be after the NEXT sentence (if any)
		if i < len(sentences)-1 {
			nextSentenceTokens := s.tokenizer.CountTokens(sentences[i+1])
			if currentTokens+nextSentenceTokens+1 > s.config.MaxChunkSize { // +1 for space
				shouldSplit = true
			}
		}

		if shouldSplit {
			currentChunk.EndChar = currentChar
			chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))

			// Start new chunk if there are more sentences
			if i < len(sentences)-1 {
				currentChunk = &chunking.TextChunk{
					Metadata:  copyMetadata(metadata),
					StartChar: currentChar,
					Content:   "",
				}
				currentTokens = 0
			} else {
				// No more sentences, clear to prevent duplicate
				currentTokens = 0
				currentChunk = nil
			}
		}
	}

	// Add final chunk if it exists and meets minimum size
	if currentChunk != nil && currentTokens >= s.config.MinChunkSize {
		currentChunk.EndChar = currentChar
		chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))
	} else if currentChunk != nil && len(chunks) > 0 && currentChunk.Content != "" {
		// Check if we can merge with previous chunk without exceeding max size
		lastChunk := chunks[len(chunks)-1]
		mergedContent := lastChunk.Content + " " + currentChunk.Content
		mergedTokens := s.tokenizer.CountTokens(mergedContent)

		if mergedTokens <= s.config.MaxChunkSize {
			// Safe to merge
			lastChunk.Content = mergedContent
			lastChunk.EndChar = currentChar
			lastChunk.TokenCount = mergedTokens
		} else {
			// Can't merge, create a new chunk even if it's small
			currentChunk.EndChar = currentChar
			chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))
		}
	}

	return chunks, nil
}

// GetConfig returns the chunker configuration
func (s *SemanticChunker) GetConfig() interface{} {
	return s.config
}

// isSemanticBoundary detects semantic boundaries between sentences
func (s *SemanticChunker) isSemanticBoundary(sentences []string, index int) bool {
	if index >= len(sentences)-1 {
		return true
	}

	currentSentence := sentences[index]
	nextSentence := sentences[index+1]

	// Check for paragraph boundaries (double newline)
	if strings.HasSuffix(currentSentence, "\n\n") {
		return true
	}

	// Check for section headers (simple heuristic)
	// Headers are often short and don't end with periods
	if len(nextSentence) < 100 && !strings.HasSuffix(nextSentence, ".") &&
		!strings.HasSuffix(nextSentence, "!") && !strings.HasSuffix(nextSentence, "?") {
		// Check if it might be a header (starts with capital, no punctuation)
		trimmed := strings.TrimSpace(nextSentence)
		if len(trimmed) > 0 && strings.ToUpper(trimmed[:1]) == trimmed[:1] {
			return true
		}
	}

	// Check for topic shift indicators
	topicShiftIndicators := []string{
		"however,", "furthermore,", "additionally,", "in conclusion,",
		"on the other hand,", "in summary,", "next,", "finally,",
		"first,", "second,", "third,", "lastly,", "moreover,",
		"nevertheless,", "consequently,", "therefore,", "thus,",
		"in contrast,", "alternatively,", "meanwhile,",
	}

	lowerNext := strings.ToLower(nextSentence)
	for _, indicator := range topicShiftIndicators {
		if strings.HasPrefix(lowerNext, indicator) {
			return true
		}
	}

	// Check for bullet points or numbered lists
	trimmedNext := strings.TrimSpace(nextSentence)
	if len(trimmedNext) > 0 {
		// Numbered list
		if len(trimmedNext) > 2 && trimmedNext[1] == '.' ||
			trimmedNext[1] == ')' && (trimmedNext[0] >= '0' && trimmedNext[0] <= '9') {
			return true
		}
		// Bullet points
		if strings.HasPrefix(trimmedNext, "•") || strings.HasPrefix(trimmedNext, "-") ||
			strings.HasPrefix(trimmedNext, "*") || strings.HasPrefix(trimmedNext, "·") {
			return true
		}
	}

	return false
}

// getOverlapText extracts overlap text from the end of content
func (s *SemanticChunker) getOverlapText(content string, overlapTokens int) string {
	if overlapTokens <= 0 || content == "" {
		return ""
	}

	sentences := s.sentenceSplitter.Split(content)
	if len(sentences) == 0 {
		return ""
	}

	overlapContent := ""
	tokenCount := 0

	// Add sentences from the end until we reach overlap size
	for i := len(sentences) - 1; i >= 0 && tokenCount < overlapTokens; i-- {
		sentence := sentences[i]
		sentTokens := s.tokenizer.CountTokens(sentence)

		// Allow 20% overflow for complete sentences
		if tokenCount+sentTokens <= int(float64(overlapTokens)*1.2) {
			if overlapContent == "" {
				overlapContent = sentence
			} else {
				overlapContent = sentence + " " + overlapContent
			}
			tokenCount += sentTokens
		} else {
			break
		}
	}

	return strings.TrimSpace(overlapContent)
}

// finalizeChunk prepares a chunk for output
func (s *SemanticChunker) finalizeChunk(chunk *chunking.TextChunk, index int) *chunking.TextChunk {
	chunk.Index = index
	chunk.TokenCount = s.tokenizer.CountTokens(chunk.Content)

	// Ensure chunk doesn't exceed max size (safety check)
	if chunk.TokenCount > s.config.MaxChunkSize {
		// Trim chunk to max size by removing words from the end
		words := strings.Fields(chunk.Content)
		for len(words) > 0 {
			testContent := strings.Join(words, " ")
			testTokens := s.tokenizer.CountTokens(testContent)
			if testTokens <= s.config.MaxChunkSize {
				chunk.Content = testContent
				chunk.TokenCount = testTokens
				break
			}
			words = words[:len(words)-1]
		}
	}

	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}
	chunk.Metadata["chunking_method"] = "semantic"
	chunk.Metadata["chunk_index"] = index
	chunk.Metadata["config"] = map[string]interface{}{
		"target_size": s.config.TargetChunkSize,
		"overlap":     s.config.OverlapSize,
	}

	return chunk
}

// copyMetadata creates a copy of the metadata map
func copyMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return make(map[string]interface{})
	}

	copied := make(map[string]interface{})
	for k, v := range metadata {
		copied[k] = v
	}
	return copied
}
