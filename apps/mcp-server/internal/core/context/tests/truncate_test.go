package tests

import (
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestTruncateOldestFirst tests the TruncateOldestFirst function
func TestTruncateOldestFirst(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name           string
		context        *models.Context
		expectedItems  int
		expectedTokens int
	}{
		{
			name: "context under max tokens",
			context: &models.Context{
				MaxTokens:     100,
				CurrentTokens: 50,
				Content: []models.ContextItem{
					{
						Role:      "user",
						Content:   "Message 1",
						Tokens:    20,
						Timestamp: time.Now().Add(-2 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Message 2",
						Tokens:    30,
						Timestamp: time.Now().Add(-1 * time.Hour),
					},
				},
			},
			expectedItems:  2,
			expectedTokens: 50,
		},
		{
			name: "context over max tokens",
			context: &models.Context{
				MaxTokens:     50,
				CurrentTokens: 100,
				Content: []models.ContextItem{
					{
						Role:      "user",
						Content:   "Message 1",
						Tokens:    30,
						Timestamp: time.Now().Add(-3 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Message 2",
						Tokens:    30,
						Timestamp: time.Now().Add(-2 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "Message 3",
						Tokens:    40,
						Timestamp: time.Now().Add(-1 * time.Hour),
					},
				},
			},
			expectedItems:  1,
			expectedTokens: 40,
		},
		{
			name: "empty context",
			context: &models.Context{
				MaxTokens:     100,
				CurrentTokens: 0,
				Content:       []models.ContextItem{},
			},
			expectedItems:  0,
			expectedTokens: 0,
		},
		{
			name: "exact token match",
			context: &models.Context{
				MaxTokens:     50,
				CurrentTokens: 50,
				Content: []models.ContextItem{
					{
						Role:      "user",
						Content:   "Message 1",
						Tokens:    20,
						Timestamp: time.Now().Add(-1 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Message 2",
						Tokens:    30,
						Timestamp: time.Now(),
					},
				},
			},
			expectedItems:  2,
			expectedTokens: 50,
		},
	}

	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy of the context to avoid modifying the test case
			contextCopy := &models.Context{
				MaxTokens:     tc.context.MaxTokens,
				CurrentTokens: tc.context.CurrentTokens,
				Content:       make([]models.ContextItem, len(tc.context.Content)),
			}
			
			// Copy content items
			for i, item := range tc.context.Content {
				contextCopy.Content[i] = item
			}
			
			// Call DoTruncateOldestFirst
			err := DoTruncateOldestFirst(contextCopy)
			
			// Assert no error
			assert.NoError(t, err)
			
			// Assert expected results
			assert.Len(t, contextCopy.Content, tc.expectedItems)
			assert.Equal(t, tc.expectedTokens, contextCopy.CurrentTokens)
			
			// Assert tokens are <= max tokens
			assert.LessOrEqual(t, contextCopy.CurrentTokens, contextCopy.MaxTokens)
		})
	}
}

// TestTruncatePreservingUser tests the TruncatePreservingUser function
func TestTruncatePreservingUser(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name           string
		context        *models.Context
		expectedTokens int
	}{
		{
			name: "context under max tokens",
			context: &models.Context{
				MaxTokens:     100,
				CurrentTokens: 50,
				Content: []models.ContextItem{
					{
						Role:      "user",
						Content:   "User Message 1",
						Tokens:    20,
						Timestamp: time.Now().Add(-2 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Assistant Message 1",
						Tokens:    30,
						Timestamp: time.Now().Add(-1 * time.Hour),
					},
				},
			},
			expectedTokens: 50,
		},
		{
			name: "assistant messages only",
			context: &models.Context{
				MaxTokens:     50,
				CurrentTokens: 100,
				Content: []models.ContextItem{
					{
						Role:      "assistant",
						Content:   "Assistant Message 1",
						Tokens:    30,
						Timestamp: time.Now().Add(-3 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Assistant Message 2",
						Tokens:    30,
						Timestamp: time.Now().Add(-2 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Assistant Message 3",
						Tokens:    40,
						Timestamp: time.Now().Add(-1 * time.Hour),
					},
				},
			},
			expectedTokens: 40, // Only the newest assistant message should remain
		},
		{
			name: "user messages only",
			context: &models.Context{
				MaxTokens:     50,
				CurrentTokens: 100,
				Content: []models.ContextItem{
					{
						Role:      "user",
						Content:   "User Message 1",
						Tokens:    30,
						Timestamp: time.Now().Add(-3 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "User Message 2",
						Tokens:    30,
						Timestamp: time.Now().Add(-2 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "User Message 3",
						Tokens:    40,
						Timestamp: time.Now().Add(-1 * time.Hour),
					},
				},
			},
			expectedTokens: 40, // Only the newest user message should remain
		},
		{
			name: "mixed messages",
			context: &models.Context{
				MaxTokens:     50,
				CurrentTokens: 130,
				Content: []models.ContextItem{
					{
						Role:      "system",
						Content:   "System Message",
						Tokens:    20,
						Timestamp: time.Now().Add(-4 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "User Message 1",
						Tokens:    20,
						Timestamp: time.Now().Add(-3 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Assistant Message 1",
						Tokens:    30,
						Timestamp: time.Now().Add(-2 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "User Message 2",
						Tokens:    30,
						Timestamp: time.Now().Add(-1 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Assistant Message 2",
						Tokens:    30,
						Timestamp: time.Now(),
					},
				},
			},
			expectedTokens: 50, // Should be <= max tokens
		},
	}

	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy of the context to avoid modifying the test case
			contextCopy := &models.Context{
				MaxTokens:     tc.context.MaxTokens,
				CurrentTokens: tc.context.CurrentTokens,
				Content:       make([]models.ContextItem, len(tc.context.Content)),
			}
			
			// Copy content items
			for i, item := range tc.context.Content {
				contextCopy.Content[i] = item
			}
			
			// Call DoTruncatePreservingUser
			err := DoTruncatePreservingUser(contextCopy)
			
			// Assert no error
			assert.NoError(t, err)
			
			// Assert tokens are <= max tokens
			assert.LessOrEqual(t, contextCopy.CurrentTokens, contextCopy.MaxTokens)
			
			// Verify content consistency
			actualTokens := 0
			for _, item := range contextCopy.Content {
				actualTokens += item.Tokens
			}
			assert.Equal(t, contextCopy.CurrentTokens, actualTokens)
		})
	}
}
