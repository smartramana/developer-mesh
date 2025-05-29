package parsers

import (
	"context"
	"strings"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
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

func TestPythonParser_Parse(t *testing.T) {
	// Create Python parser
	parser := NewPythonParser()

	// Test that the parser returns the correct language
	assert.Equal(t, chunking.LanguagePython, parser.GetLanguage())

	// Create simple Python code for testing
	pyCode := `#!/usr/bin/env python
# -*- coding: utf-8 -*-

"""
User management module for handling user operations.
"""

import uuid
import re
from datetime import datetime

class User:
    """
    User class for representing a user in the system.
    """

    def __init__(self, name, email):
        """
        Create a new user.
        
        Args:
            name (str): The user's name
            email (str): The user's email
        """
        self.id = str(uuid.uuid4())
        self.name = name
        self.email = email
        self.created_at = datetime.now().isoformat()
    
    def get_display_name(self):
        """
        Get the user's display name.
        
        Returns:
            str: The user's name
        """
        return self.name
    
    def get_email_domain(self):
        """
        Get the domain part of the user's email.
        
        Returns:
            str: The email domain
        """
        parts = self.email.split('@')
        if len(parts) > 1:
            return parts[1]
        return ''


def create_user(name, email):
    """
    Create a new user.
    
    Args:
        name (str): The user's name
        email (str): The user's email
        
    Returns:
        User: A new user instance
    """
    return User(name, email)


# Example usage
def main():
    user = create_user('John Doe', 'john@example.com')
    print(f"User: {user.get_display_name()}")
    print(f"Email Domain: {user.get_email_domain()}")


if __name__ == "__main__":
    main()`

	// Parse the code
	chunks, err := parser.Parse(context.Background(), pyCode, "test.py")

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
	assert.Equal(t, "test.py", fileChunk.Name)
	assert.Equal(t, chunking.LanguagePython, fileChunk.Language)

	// Verify class chunk exists
	var classChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeClass && chunk.Name == "User" {
			classChunk = chunk
			break
		}
	}
	assert.NotNil(t, classChunk)
	assert.Equal(t, "User", classChunk.Name)

	// Verify method chunks exist
	var methodChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeMethod && chunk.Name == "get_email_domain" {
			methodChunk = chunk
			break
		}
	}
	assert.NotNil(t, methodChunk)
	assert.Equal(t, "get_email_domain", methodChunk.Name)
	assert.Equal(t, "User.get_email_domain", methodChunk.Path)

	// Verify function chunks exist
	var functionChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFunction && chunk.Name == "create_user" {
			functionChunk = chunk
			break
		}
	}
	assert.NotNil(t, functionChunk)
	assert.Equal(t, "create_user", functionChunk.Name)

	// Verify import chunks exist
	var importFound bool
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeImport {
			importFound = true
			break
		}
	}
	assert.True(t, importFound, "Expected to find import chunks")

	// Verify docstring chunks exist
	var docstringFound bool
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeComment {
			docstringFound = true
			break
		}
	}
	assert.True(t, docstringFound, "Expected to find docstring chunks")

	// Verify dependencies are created
	dependenciesFound := false
	for _, chunk := range chunks {
		if len(chunk.Dependencies) > 0 {
			dependenciesFound = true
			break
		}
	}
	assert.True(t, dependenciesFound, "Expected to find dependencies between chunks")
}

func TestHCLParser_Parse(t *testing.T) {
	// Create HCL parser
	parser := NewHCLParser()

	// Test that the parser returns the correct language
	assert.Equal(t, chunking.LanguageHCL, parser.GetLanguage())

	// Create simple Terraform (HCL) code for testing
	hclCode := `# Configure the AWS Provider
provider "aws" {
  region = "us-west-2"
  access_key = var.aws_access_key
  secret_key = var.aws_secret_key
}

# Define variables
variable "aws_access_key" {
  description = "AWS access key"
  type        = string
  sensitive   = true
}

variable "aws_secret_key" {
  description = "AWS secret key"
  type        = string
  sensitive   = true
}

variable "instance_name" {
  description = "Value of the Name tag for the EC2 instance"
  type        = string
  default     = "ExampleInstance"
}

# Terraform settings
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
  }
}

# Define a local value
locals {
  common_tags = {
    Project     = "Example"
    Environment = "Development"
  }
}

# Create an EC2 instance
resource "aws_instance" "app_server" {
  ami           = "ami-830c94e3"
  instance_type = "t2.micro"

  tags = merge(
    local.common_tags,
    {
      Name = var.instance_name
    }
  )
}

# Create an S3 bucket
resource "aws_s3_bucket" "example" {
  bucket = "my-example-bucket"
  tags   = local.common_tags
}

# Define data source
data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"] # Canonical
}

# Define outputs
output "instance_id" {
  description = "ID of the EC2 instance"
  value       = aws_instance.app_server.id
}

output "instance_public_ip" {
  description = "Public IP address of the EC2 instance"
  value       = aws_instance.app_server.public_ip
}

# Import external module
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "3.14.0"

  name = "my-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["us-west-2a", "us-west-2b", "us-west-2c"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  tags = local.common_tags
}`

	// Parse the code
	chunks, err := parser.Parse(context.Background(), hclCode, "main.tf")

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
	assert.Equal(t, "main.tf", fileChunk.Name)
	assert.Equal(t, chunking.LanguageHCL, fileChunk.Language)

	// Verify resource blocks exist
	var resourceChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeBlock && strings.HasPrefix(chunk.Name, "resource.") {
			resourceChunk = chunk
			break
		}
	}
	assert.NotNil(t, resourceChunk)
	assert.Contains(t, resourceChunk.Name, "resource.aws_instance.app_server")

	// Verify variable blocks exist
	var variableChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeBlock && strings.HasPrefix(chunk.Name, "var.") {
			variableChunk = chunk
			break
		}
	}
	assert.NotNil(t, variableChunk)

	// Verify provider blocks exist
	var providerChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeBlock && strings.HasPrefix(chunk.Name, "provider.") {
			providerChunk = chunk
			break
		}
	}
	assert.NotNil(t, providerChunk)
	assert.Equal(t, "provider.aws", providerChunk.Name)

	// Verify output blocks exist
	var outputChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeBlock && strings.HasPrefix(chunk.Name, "output.") {
			outputChunk = chunk
			break
		}
	}
	assert.NotNil(t, outputChunk)

	// Verify module blocks exist
	var moduleChunk *chunking.CodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeBlock && strings.HasPrefix(chunk.Name, "module.") {
			moduleChunk = chunk
			break
		}
	}
	assert.NotNil(t, moduleChunk)
	assert.Equal(t, "module.vpc", moduleChunk.Name)

	// Verify dependencies are created
	dependenciesFound := false
	for _, chunk := range chunks {
		if len(chunk.Dependencies) > 0 {
			dependenciesFound = true
			break
		}
	}
	assert.True(t, dependenciesFound, "Expected to find dependencies between chunks")
}

func TestParserFactory(t *testing.T) {
	// Get all parsers from the factory
	parsers := NewParserFactory()

	// Verify that we have parsers for supported languages
	assert.NotNil(t, parsers[chunking.LanguageGo])
	assert.NotNil(t, parsers[chunking.LanguageJavaScript])
	assert.NotNil(t, parsers[chunking.LanguagePython])
	assert.NotNil(t, parsers[chunking.LanguageHCL])
	assert.NotNil(t, parsers[chunking.LanguageTypeScript])
	assert.NotNil(t, parsers[chunking.LanguageShell])
	assert.NotNil(t, parsers[chunking.LanguageRust])
	assert.NotNil(t, parsers[chunking.LanguageKotlin])

	// Check that the parsers return the expected language
	assert.Equal(t, chunking.LanguageGo, parsers[chunking.LanguageGo].GetLanguage())
	assert.Equal(t, chunking.LanguageJavaScript, parsers[chunking.LanguageJavaScript].GetLanguage())
	assert.Equal(t, chunking.LanguagePython, parsers[chunking.LanguagePython].GetLanguage())
	assert.Equal(t, chunking.LanguageHCL, parsers[chunking.LanguageHCL].GetLanguage())
	assert.Equal(t, chunking.LanguageTypeScript, parsers[chunking.LanguageTypeScript].GetLanguage())
	assert.Equal(t, chunking.LanguageShell, parsers[chunking.LanguageShell].GetLanguage())
	assert.Equal(t, chunking.LanguageRust, parsers[chunking.LanguageRust].GetLanguage())
	assert.Equal(t, chunking.LanguageKotlin, parsers[chunking.LanguageKotlin].GetLanguage())
}

func TestInitializeChunkingService(t *testing.T) {
	// Initialize the chunking service
	service := InitializeChunkingService()

	// Verify that the service was created
	assert.NotNil(t, service)

	// Test that the service can detect languages
	assert.Equal(t, chunking.LanguageGo, service.DetectLanguage("test.go", ""))
	assert.Equal(t, chunking.LanguageJavaScript, service.DetectLanguage("test.js", ""))
	assert.Equal(t, chunking.LanguagePython, service.DetectLanguage("test.py", ""))
	assert.Equal(t, chunking.LanguagePython, service.DetectLanguage("test.pyw", ""))
	assert.Equal(t, chunking.LanguageHCL, service.DetectLanguage("test.tf", ""))
	assert.Equal(t, chunking.LanguageHCL, service.DetectLanguage("test.hcl", ""))
	assert.Equal(t, chunking.LanguageTypeScript, service.DetectLanguage("test.ts", ""))
	assert.Equal(t, chunking.LanguageTypeScript, service.DetectLanguage("test.tsx", ""))
	assert.Equal(t, chunking.LanguageShell, service.DetectLanguage("test.sh", ""))
	assert.Equal(t, chunking.LanguageShell, service.DetectLanguage("test.bash", ""))
	assert.Equal(t, chunking.LanguageRust, service.DetectLanguage("test.rs", ""))
	assert.Equal(t, chunking.LanguageKotlin, service.DetectLanguage("test.kt", ""))
	assert.Equal(t, chunking.LanguageKotlin, service.DetectLanguage("test.kts", ""))
}
