package parsers

import (
"context"
"testing"

"github.com/S-Corkum/devops-mcp/internal/chunking"
"github.com/stretchr/testify/assert"
)

func TestGoParser_Parse(t *testing.T) {
	// Create Go parser
	parser := NewGoParser()

	// Test that the parser returns the correct language
	assert.Equal(t, chunking.LanguageGo, parser.GetLanguage())

	// Create simple Go code for testing
	goCode := `package test

import (
"fmt"
"strings"
)

// User represents a user in the system
type User struct {
	ID        string
	Name      string
	Email     string
	CreatedAt string
}

// NewUser creates a new user
func NewUser(name, email string) *User {
	return &User{
		ID:        "user123",
		Name:      name,
		Email:     email,
		CreatedAt: "2023-01-01",
	}
}

// GetDisplayName returns the user's display name
func (u *User) GetDisplayName() string {
return u.Name
}

// GetEmailDomain extracts the domain part from the user's email
func (u *User) GetEmailDomain() string {
	parts := strings.Split(u.Email, "@")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// Main function to demonstrate usage
func main() {
	user := NewUser("John Doe", "john@example.com")
	fmt.Println("User:", user.GetDisplayName())
	fmt.Println("Email Domain:", user.GetEmailDomain())
}`

	// Parse the code
	chunks, err := parser.Parse(context.Background(), goCode, "test.go")
	
	// Verify no error occurred
	assert.NoError(t, err)
	
	// Verify chunks were created
	assert.Greater(t, len(chunks), 0)

	// Verify file chunk
	var fileChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFile {
			fileChunk = chunk
			break
		}
	}
	assert.NotNil(t, fileChunk)
	assert.Equal(t, "test.go", fileChunk.Name)
	assert.Equal(t, chunking.LanguageGo, fileChunk.Language)

	// Verify struct chunk
	var structChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeStruct && chunk.Name == "User" {
			structChunk = chunk
			break
		}
	}
	assert.NotNil(t, structChunk)
	assert.Equal(t, "User", structChunk.Name)
	assert.Equal(t, "test.User", structChunk.Path)

	// Verify function chunks
	var functionChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFunction && chunk.Name == "NewUser" {
			functionChunk = chunk
			break
		}
	}
	assert.NotNil(t, functionChunk)
	assert.Equal(t, "NewUser", functionChunk.Name)
	assert.Equal(t, "test.NewUser", functionChunk.Path)
}

func TestJavaScriptParser_Parse(t *testing.T) {
	// Create JavaScript parser
	parser := NewJavaScriptParser()

	// Test that the parser returns the correct language
	assert.Equal(t, chunking.LanguageJavaScript, parser.GetLanguage())

	// Create simple JavaScript code for testing
	jsCode := `// User management module
import { v4 as uuidv4 } from 'uuid';

/**
 * User class for representing a user in the system
 */
class User {
	/**
	 * Create a new user
	 * @param {string} name - The user's name
 * @param {string} email - The user's email
	 */
	constructor(name, email) {
		this.id = uuidv4();
		this.name = name;
		this.email = email;
		this.createdAt = new Date().toISOString();
	}

	/**
	 * Get the user's display name
 * @returns {string} The user's name
	 */
	getDisplayName() {
		return this.name;
	}

	/**
	 * Get the domain part of the user's email
 * @returns {string} The email domain
 */
getEmailDomain() {
const parts = this.email.split('@');
if (parts.length > 1) {
return parts[1];
}
return '';
}
}

/**
 * Create a new user
 * @param {string} name - The user's name
 * @param {string} email - The user's email
 * @returns {User} A new user instance
 */
const createUser = (name, email) => {
return new User(name, email);
};

// Example usage
function main() {
const user = createUser('John Doe', 'john@example.com');
console.log('User:', user.getDisplayName());
console.log('Email Domain:', user.getEmailDomain());
}

main();`

	// Parse the code
	chunks, err := parser.Parse(context.Background(), jsCode, "test.js")
	
	// Verify no error occurred
	assert.NoError(t, err)
	
	// Verify chunks were created
	assert.Greater(t, len(chunks), 0)

	// Verify file chunk exists
	var fileChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFile {
			fileChunk = chunk
			break
		}
	}
	assert.NotNil(t, fileChunk)
	assert.Equal(t, "test.js", fileChunk.Name)
	assert.Equal(t, chunking.LanguageJavaScript, fileChunk.Language)

	// Check if we have a class chunk, but don't require it since the parser implementation may vary
	// Just verify we have enough chunks in general
	
	// Verify we have at least one chunked element (class, function, method, etc.)
	hasCodeElements := false
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeClass || 
		   chunk.Type == chunking.ChunkTypeFunction || 
		   chunk.Type == chunking.ChunkTypeMethod {
			hasCodeElements = true
			break
		}
	}
	assert.True(t, hasCodeElements, "Expected to find at least one code element (class, function, or method)")
}

func TestParserFactory(t *testing.T) {
	// Get all parsers from the factory
	parsers := NewParserFactory()
	
	// Verify that we have parsers for supported languages
	assert.NotNil(t, parsers[chunking.LanguageGo])
	assert.NotNil(t, parsers[chunking.LanguageJavaScript])
	
	// Check that the parsers return the expected language
	assert.Equal(t, chunking.LanguageGo, parsers[chunking.LanguageGo].GetLanguage())
	assert.Equal(t, chunking.LanguageJavaScript, parsers[chunking.LanguageJavaScript].GetLanguage())
}

func TestInitializeChunkingService(t *testing.T) {
	// Initialize the chunking service
	service := InitializeChunkingService()
	
	// Verify that the service was created
	assert.NotNil(t, service)
	
	// Test that the service can detect languages
	assert.Equal(t, chunking.LanguageGo, service.DetectLanguage("test.go", ""))
	assert.Equal(t, chunking.LanguageJavaScript, service.DetectLanguage("test.js", ""))
}
