package text

import (
	"strings"
	"unicode"
)

// SentenceSplitter splits text into sentences
type SentenceSplitter interface {
	Split(text string) []string
}

// DefaultSentenceSplitter implements a rule-based sentence splitter
type DefaultSentenceSplitter struct {
	abbreviations map[string]bool
}

// NewSentenceSplitter creates a new sentence splitter
func NewSentenceSplitter() SentenceSplitter {
	return &DefaultSentenceSplitter{
		abbreviations: getCommonAbbreviations(),
	}
}

// Split splits text into sentences
func (s *DefaultSentenceSplitter) Split(text string) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}

	var sentences []string
	var currentSentence strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		// Check for opening parenthesis that should start a new sentence
		if i > 0 && runes[i] == '(' && currentSentence.Len() > 0 {
			// Check if we should split before this parenthesis
			// Look at what's in the current sentence buffer
			currentText := currentSentence.String()
			trimmed := strings.TrimSpace(currentText)

			// If current buffer ends with punctuation + quote, split here
			if len(trimmed) > 0 {
				lastChar := trimmed[len(trimmed)-1]
				if lastChar == '"' || lastChar == '\'' || lastChar == '!' || lastChar == '?' {
					// Save current sentence
					if trimmed != "" {
						sentences = append(sentences, trimmed)
					}
					currentSentence.Reset()
				}
			}
		}

		currentSentence.WriteRune(runes[i])

		// Check for paragraph boundaries (double newline)
		if i+1 < len(runes) && runes[i] == '\n' && runes[i+1] == '\n' {
			// Add the second newline
			currentSentence.WriteRune(runes[i+1])
			i++ // skip the second newline in loop

			sentence := currentSentence.String()
			if strings.TrimSpace(sentence) != "" {
				sentences = append(sentences, sentence)
			}
			currentSentence.Reset()
			continue
		}

		// Check for sentence endings
		if s.isSentenceEnd(runes, i) {
			// Look ahead for continuation
			if !s.isContinuation(runes, i) {
				// Check if we need to include following quotes/brackets/parentheses
				endPos := i
				for endPos+1 < len(runes) && (runes[endPos+1] == '"' || runes[endPos+1] == '\'' ||
					runes[endPos+1] == ')' || runes[endPos+1] == ']' || runes[endPos+1] == '}') {
					currentSentence.WriteRune(runes[endPos+1])
					endPos++
					i = endPos // Skip these characters in the main loop
				}

				sentence := strings.TrimSpace(currentSentence.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				currentSentence.Reset()
			}
		}
	}

	// Add any remaining text as a sentence
	if currentSentence.Len() > 0 {
		sentence := strings.TrimSpace(currentSentence.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// isSentenceEnd checks if the current position is a sentence ending
func (s *DefaultSentenceSplitter) isSentenceEnd(runes []rune, pos int) bool {
	if pos >= len(runes) {
		return false
	}

	r := runes[pos]

	// Check for sentence-ending punctuation
	if r != '.' && r != '!' && r != '?' {
		return false
	}

	// For periods, check if it's part of an abbreviation
	if r == '.' {
		// Check for ellipsis first (three dots)
		if pos >= 2 && runes[pos-1] == '.' && runes[pos-2] == '.' {
			// This is the third dot of an ellipsis
			return false
		}
		if pos >= 1 && pos+1 < len(runes) && runes[pos-1] == '.' && runes[pos+1] == '.' {
			// This is the middle dot of an ellipsis
			return false
		}
		if pos+2 < len(runes) && runes[pos+1] == '.' && runes[pos+2] == '.' {
			// This is the first dot of an ellipsis
			return false
		}

		// Look back to find the word before the period
		wordEnd := pos
		wordStart := pos - 1
		// Skip back to find start of word
		for wordStart >= 0 && !unicode.IsSpace(runes[wordStart]) && runes[wordStart] != '.' {
			wordStart--
		}
		// Adjust wordStart position
		if wordStart < 0 {
			wordStart = 0
		} else if wordStart < len(runes) && (unicode.IsSpace(runes[wordStart]) || runes[wordStart] == '.') {
			wordStart++
		}

		word := string(runes[wordStart:wordEnd])
		// fmt.Printf("DEBUG: Checking abbreviation %q at pos %d\n", word, pos)
		if s.isAbbreviation(word) {
			// Check if it's U.S.A. style abbreviation (multiple periods)
			if pos+1 < len(runes) && runes[pos+1] == '.' {
				return false // Part of multi-period abbreviation
			}

			// For single-period abbreviations, we need more context
			// Check what follows to determine if it's really a sentence boundary
			if pos+1 < len(runes) {
				// Skip any spaces after the period
				nextPos := pos + 1
				for nextPos < len(runes) && unicode.IsSpace(runes[nextPos]) {
					nextPos++
				}

				// If we have enough context, check if next word looks like sentence start
				if nextPos < len(runes) {
					// Check for pronouns and common sentence starters that indicate new sentence
					nextWord := s.getNextWord(runes, nextPos)
					if s.isSentenceStarter(nextWord) {
						return true // Split even after abbreviation
					}
					// For titles (Dr., Mr., etc), don't split before names
					lowerWord := strings.ToLower(word)
					if lowerWord == "dr" || lowerWord == "mr" || lowerWord == "mrs" || lowerWord == "ms" ||
						lowerWord == "prof" || lowerWord == "sr" || lowerWord == "jr" {
						return false // Don't split after titles
					}

					// Also split if next word is capitalized and not an abbreviation itself
					if nextWord != "" && unicode.IsUpper(runes[nextPos]) && !s.isAbbreviation(strings.ToLower(nextWord)) {
						return true
					}
				}
			}

			return false
		}

		// Check for decimal numbers (e.g., 3.14)
		if pos > 0 && unicode.IsDigit(runes[pos-1]) &&
			pos+1 < len(runes) && unicode.IsDigit(runes[pos+1]) {
			return false
		}

		// Check if we're inside parentheses - don't split on period inside parens
		// unless followed by closing paren
		parenDepth := 0
		for j := 0; j < pos; j++ {
			switch runes[j] {
			case '(':
				parenDepth++
			case ')':
				parenDepth--
			}
		}
		if parenDepth > 0 && pos+1 < len(runes) && runes[pos+1] != ')' {
			return false // Don't split inside parentheses
		}
	}

	// Look ahead to see if followed by space and capital letter or end of text
	if pos+1 >= len(runes) {
		return true
	}

	// Check for closing quotes or parentheses after punctuation
	nextPos := pos + 1
	for nextPos < len(runes) && (runes[nextPos] == '"' || runes[nextPos] == '\'' ||
		runes[nextPos] == ')' || runes[nextPos] == ']' ||
		runes[nextPos] == '}') {
		nextPos++
	}

	// Must be followed by whitespace
	if nextPos < len(runes) && !unicode.IsSpace(runes[nextPos]) {
		return false
	}

	// Skip whitespace
	for nextPos < len(runes) && unicode.IsSpace(runes[nextPos]) {
		nextPos++
	}

	// If at end of text, it's a sentence end
	if nextPos >= len(runes) {
		return true
	}

	// Check if next word starts with capital letter
	// But be more lenient after ellipsis
	if pos >= 2 && runes[pos] == '.' && runes[pos-1] == '.' && runes[pos-2] == '.' {
		// After ellipsis, only split if there's a clear sentence pattern
		// (e.g., two spaces, or specific punctuation patterns)
		spaceCount := 0
		for i := pos + 1; i < nextPos && i < len(runes); i++ {
			if unicode.IsSpace(runes[i]) {
				spaceCount++
			}
		}
		// Only consider it a sentence end if there are multiple spaces or newlines
		if spaceCount < 2 && !strings.ContainsRune(string(runes[pos+1:nextPos]), '\n') {
			return false
		}
	}

	return unicode.IsUpper(runes[nextPos])
}

// isContinuation checks if the sentence continues after punctuation
func (s *DefaultSentenceSplitter) isContinuation(runes []rune, pos int) bool {
	if pos+1 >= len(runes) {
		return false
	}

	// Skip any quotes or brackets after punctuation
	nextPos := pos + 1
	for nextPos < len(runes) && (runes[nextPos] == '"' || runes[nextPos] == '\'' ||
		runes[nextPos] == ')' || runes[nextPos] == ']') {
		nextPos++
	}

	// Check for immediate continuation without space (e.g., "U.S.A.")
	if nextPos < len(runes) && !unicode.IsSpace(runes[nextPos]) {
		return true
	}

	return false
}

// isAbbreviation checks if a word is a common abbreviation
func (s *DefaultSentenceSplitter) isAbbreviation(word string) bool {
	word = strings.ToLower(strings.TrimSpace(word))
	return s.abbreviations[word]
}

// getNextWord extracts the next word starting at position
func (s *DefaultSentenceSplitter) getNextWord(runes []rune, start int) string {
	if start >= len(runes) {
		return ""
	}

	// Find end of word
	end := start
	for end < len(runes) && !unicode.IsSpace(runes[end]) && !unicode.IsPunct(runes[end]) {
		end++
	}

	return string(runes[start:end])
}

// isSentenceStarter checks if a word is likely to start a new sentence
func (s *DefaultSentenceSplitter) isSentenceStarter(word string) bool {
	// Common pronouns and sentence starters
	starters := map[string]bool{
		"he": true, "she": true, "it": true, "they": true, "we": true, "you": true, "i": true,
		"the": true, "a": true, "an": true, "this": true, "that": true, "these": true, "those": true,
		"my": true, "your": true, "his": true, "her": true, "our": true, "their": true,
		"but": true, "and": true, "or": true, "yet": true, "so": true, "for": true, "nor": true,
		"however": true, "therefore": true, "moreover": true, "furthermore": true,
		"although": true, "though": true, "while": true, "when": true, "where": true,
		"if": true, "because": true, "since": true, "after": true, "before": true,
	}

	return starters[strings.ToLower(word)]
}

// getCommonAbbreviations returns a map of common abbreviations
func getCommonAbbreviations() map[string]bool {
	return map[string]bool{
		// Titles
		"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
		"sr": true, "jr": true, "ph.d": true, "m.d": true, "b.a": true,
		"m.a": true, "b.s": true, "m.s": true,

		// Common abbreviations
		"inc": true, "corp": true, "co": true, "ltd": true, "llc": true,
		"vs": true, "etc": true, "i.e": true, "e.g": true, "cf": true,
		"al": true, "et": true,

		// Time
		"jan": true, "feb": true, "mar": true, "apr": true, "jun": true,
		"jul": true, "aug": true, "sep": true, "sept": true, "oct": true,
		"nov": true, "dec": true,
		"mon": true, "tue": true, "wed": true, "thu": true, "fri": true,
		"sat": true, "sun": true,

		// Measurements
		"ft": true, "in": true, "cm": true, "mm": true, "km": true,
		"kg": true, "lb": true, "oz": true, "pt": true, "qt": true,

		// Geography
		"st": true, "ave": true, "blvd": true, "rd": true, "u.s": true,
		"u.k": true, "u.s.a": true,

		// Tech abbreviations
		"api": true, "sdk": true, "ui": true, "ux": true, "db": true,
		"os": true, "cpu": true, "gpu": true, "ram": true, "ssd": true,
		"http": true, "https": true, "ftp": true, "ssh": true, "oauth": true,
	}
}
