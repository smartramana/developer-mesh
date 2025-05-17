package parsers

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
	"github.com/stretchr/testify/assert"
)

func TestKotlinParser_Parse(t *testing.T) {
	// Create Kotlin parser
	parser := NewKotlinParser()

	// Test that the parser returns the correct language
	assert.Equal(t, chunking.LanguageKotlin, parser.GetLanguage())

	// Create simple Kotlin code for testing
	kotlinCode := `package com.example.app

import kotlin.collections.List
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity

/**
 * User data class representing a user in the system
 * @property id User's unique identifier
 * @property name User's full name
 * @property email User's email address
 * @property createdAt Date when the user was created
 */
data class User(
    val id: String,
    val name: String,
    val email: String,
    val createdAt: String
)

/**
 * Creates a new user with the given name and email
 * @param name User's full name
 * @param email User's email address
 * @return A new User instance
 */
fun createUser(name: String, email: String): User {
    return User(
        id = "user123",
        name = name,
        email = email,
        createdAt = "2023-01-01"
    )
}

/**
 * Main activity for the application
 */
class MainActivity : AppCompatActivity() {
    // Property declaration
    private lateinit var user: User
    
    /**
     * Called when the activity is first created
     */
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        
        // Create a user
        user = createUser("John Doe", "john@example.com")
        println("User: ${user.name}")
        println("Email Domain: ${getEmailDomain(user.email)}")
    }
    
    /**
     * Gets the domain part of an email address
     */
    private fun getEmailDomain(email: String): String {
        val parts = email.split("@")
        return if (parts.size > 1) parts[1] else ""
    }
}

/**
 * Helper object with utility functions
 */
object UserUtils {
    /**
     * Validates if an email is in the correct format
     */
    fun isValidEmail(email: String): Boolean {
        return email.contains("@") && email.contains(".")
    }
    
    /**
     * Gets the username part of an email
     */
    fun getUsernameFromEmail(email: String): String {
        return ""
    }
}`

	// Parse the code
	chunks, err := parser.Parse(context.Background(), kotlinCode, "test.kt")
	
	// Verify no error occurred
	assert.NoError(t, err)
	
	// At minimum, we should have the file chunk
	assert.GreaterOrEqual(t, len(chunks), 1, "Should have at least one chunk")

	// Verify file chunk is present
	var fileChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFile {
			fileChunk = chunk
			break
		}
	}

	assert.NotNil(t, fileChunk, "Should have a file chunk")
	
	if fileChunk != nil {
		// Verify basic file chunk properties
		assert.Equal(t, "test.kt", fileChunk.Name)
		assert.Equal(t, chunking.LanguageKotlin, fileChunk.Language)
		
		// Check if chunking method metadata is set
		method, ok := fileChunk.Metadata["chunking_method"]
		assert.True(t, ok, "Should have chunking_method in metadata")
		assert.NotEqual(t, "", method, "Chunking method should not be empty")
	}
}
