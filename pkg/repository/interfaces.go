// Package repository defines interfaces for data access operations
// This file provides backward compatibility with the new subpackage structure
package repository

import (
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/S-Corkum/devops-mcp/pkg/repository/model"
	"github.com/S-Corkum/devops-mcp/pkg/repository/search"
	"github.com/S-Corkum/devops-mcp/pkg/repository/vector"
)

//-------------------------------------------------------
// Vector Embedding Type and Interface
//-------------------------------------------------------

// Embedding represents vector embedding stored in the database
// It maintains backward compatibility with the vector package
type Embedding = vector.Embedding

// VectorAPIRepository defines the interface for vector operations
type VectorAPIRepository = vector.Repository

//-------------------------------------------------------
// Agent Interface
//-------------------------------------------------------

// AgentRepository defines the interface for agent operations
type AgentRepository = agent.Repository

//-------------------------------------------------------
// Model Interface
//-------------------------------------------------------

// ModelRepository defines the interface for model operations
type ModelRepository = model.Repository

//-------------------------------------------------------
// Search Types and Interface
//-------------------------------------------------------

// SearchOptions defines options for search operations
type SearchOptions = search.SearchOptions

// SearchFilter defines a filter for search operations
type SearchFilter = search.SearchFilter

// SearchSort defines a sort order for search operations
type SearchSort = search.SearchSort

// SearchResults contains results from a search operation
type SearchResults = search.SearchResults

// SearchResult represents a single result item from a search
type SearchResult = search.SearchResult

// SearchRepository defines the interface for search operations
type SearchRepository = search.Repository
