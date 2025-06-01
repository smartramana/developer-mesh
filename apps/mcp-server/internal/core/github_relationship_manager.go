package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/models/relationship"
	"github.com/S-Corkum/devops-mcp/pkg/storage"
)

// GitHubRelationshipManager manages relationships between GitHub entities
type GitHubRelationshipManager struct {
	relationshipService relationship.Service
	contentManager      *GitHubContentManager
}

// NewGitHubRelationshipManager creates a new relationship manager for GitHub content
func NewGitHubRelationshipManager(relationshipService relationship.Service, contentManager *GitHubContentManager) *GitHubRelationshipManager {
	return &GitHubRelationshipManager{
		relationshipService: relationshipService,
		contentManager:      contentManager,
	}
}

// ProcessContentRelationships processes relationships for stored content
func (m *GitHubRelationshipManager) ProcessContentRelationships(
	ctx context.Context,
	metadata *storage.ContentMetadata,
	content []byte,
) error {
	// Skip if metadata is missing
	if metadata == nil {
		return fmt.Errorf("content metadata is required")
	}

	// Create entity ID for this content
	entityID := models.EntityIDFromContentMetadata(
		string(metadata.ContentType),
		metadata.Owner,
		metadata.Repo,
		metadata.ContentID,
	)

	// Process based on content type
	switch metadata.ContentType {
	case storage.ContentTypeIssue:
		return m.processIssueRelationships(ctx, entityID, metadata, content)
	case storage.ContentTypePullRequest:
		return m.processPullRequestRelationships(ctx, entityID, metadata, content)
	case storage.ContentTypeCommit:
		return m.processCommitRelationships(ctx, entityID, metadata, content)
	case storage.ContentTypeFile:
		return m.processFileRelationships(ctx, entityID, metadata, content)
	case storage.ContentTypeComment:
		return m.processCommentRelationships(ctx, entityID, metadata, content)
	}

	return nil
}

// processIssueRelationships processes relationships for an issue
func (m *GitHubRelationshipManager) processIssueRelationships(
	ctx context.Context,
	issueEntity models.EntityID,
	metadata *storage.ContentMetadata,
	content []byte,
) error {
	// Extract issue data from metadata
	issueData, ok := metadata.GetMetadata()["issue"].(map[string]interface{})
	if !ok {
		return nil // No issue data available
	}

	// Process mentions of other issues
	if linkedIssues, ok := issueData["linked_issues"].([]interface{}); ok {
		for _, linkedIssue := range linkedIssues {
			if issueNum, ok := linkedIssue.(string); ok {
				// Create entity ID for linked issue
				linkedIssueEntity := models.NewEntityID(
					models.EntityTypeIssue,
					metadata.Owner,
					metadata.Repo,
					issueNum,
				)

				// Create bidirectional relationship
				err := m.relationshipService.CreateBidirectionalRelationship(
					ctx,
					models.RelationshipTypeReferences,
					issueEntity,
					linkedIssueEntity,
					0.8,
					map[string]interface{}{
						"relationship_source": "linked_issues",
					},
				)
				if err != nil {
					return fmt.Errorf("failed to create relationship to linked issue: %w", err)
				}
			}
		}
	}

	// Process mentions of users
	if mentions, ok := issueData["mentions"].([]interface{}); ok {
		for _, mention := range mentions {
			if username, ok := mention.(string); ok {
				// Create entity ID for user
				userEntity := models.NewEntityID(
					models.EntityTypeUser,
					username,
					"",
					username,
				)

				// Create relationship
				err := m.relationshipService.CreateRelationship(
					ctx,
					models.NewEntityRelationship(
						models.RelationshipTypeReferences,
						issueEntity,
						userEntity,
						models.DirectionOutgoing,
						0.5,
					).WithContext("Mentioned in issue description"),
				)
				if err != nil {
					return fmt.Errorf("failed to create relationship to mentioned user: %w", err)
				}
			}
		}
	}

	// Process relationship to repository
	repoEntity := models.NewEntityID(
		models.EntityTypeRepository,
		metadata.Owner,
		metadata.Repo,
		metadata.Repo,
	)

	// Create relationship to repository
	err := m.relationshipService.CreateRelationship(
		ctx,
		models.NewEntityRelationship(
			models.RelationshipTypeContains,
			repoEntity,
			issueEntity,
			models.DirectionOutgoing,
			1.0,
		).WithContext("Repository contains issue"),
	)
	if err != nil {
		return fmt.Errorf("failed to create relationship to repository: %w", err)
	}

	// If content is available, extract file references from issue body
	if len(content) > 0 {
		fileRefs := extractFileReferences(string(content))
		for _, fileRef := range fileRefs {
			// Create entity ID for file
			fileEntity := models.NewEntityID(
				models.EntityTypeFile,
				metadata.Owner,
				metadata.Repo,
				fileRef,
			)

			// Create relationship
			err := m.relationshipService.CreateRelationship(
				ctx,
				models.NewEntityRelationship(
					models.RelationshipTypeReferences,
					issueEntity,
					fileEntity,
					models.DirectionOutgoing,
					0.6,
				).WithContext("Referenced in issue description"),
			)
			if err != nil {
				return fmt.Errorf("failed to create relationship to file: %w", err)
			}
		}
	}

	return nil
}

// processPullRequestRelationships processes relationships for a pull request
func (m *GitHubRelationshipManager) processPullRequestRelationships(
	ctx context.Context,
	prEntity models.EntityID,
	metadata *storage.ContentMetadata,
	content []byte,
) error {
	// Extract PR data from metadata
	prData, ok := metadata.GetMetadata()["pull_request"].(map[string]interface{})
	if !ok {
		return nil // No PR data available
	}

	// Process linked issues
	if closesIssues, ok := prData["closes_issues"].([]interface{}); ok {
		for _, issueRef := range closesIssues {
			if issueNum, ok := issueRef.(string); ok {
				// Create entity ID for linked issue
				linkedIssueEntity := models.NewEntityID(
					models.EntityTypeIssue,
					metadata.Owner,
					metadata.Repo,
					issueNum,
				)

				// Create relationship
				err := m.relationshipService.CreateRelationship(
					ctx,
					models.NewEntityRelationship(
						models.RelationshipTypeModifies,
						prEntity,
						linkedIssueEntity,
						models.DirectionOutgoing,
						0.9,
					).WithContext("PR closes this issue"),
				)
				if err != nil {
					return fmt.Errorf("failed to create relationship to closed issue: %w", err)
				}
			}
		}
	}

	// Process changed files
	if changedFiles, ok := prData["changed_files"].([]interface{}); ok {
		for _, fileRef := range changedFiles {
			if filePath, ok := fileRef.(string); ok {
				// Create entity ID for file
				fileEntity := models.NewEntityID(
					models.EntityTypeFile,
					metadata.Owner,
					metadata.Repo,
					filePath,
				)

				// Create relationship
				err := m.relationshipService.CreateRelationship(
					ctx,
					models.NewEntityRelationship(
						models.RelationshipTypeModifies,
						prEntity,
						fileEntity,
						models.DirectionOutgoing,
						0.95,
					).WithContext("PR modifies this file"),
				)
				if err != nil {
					return fmt.Errorf("failed to create relationship to modified file: %w", err)
				}
			}
		}
	}

	// Process relationship to repository
	repoEntity := models.NewEntityID(
		models.EntityTypeRepository,
		metadata.Owner,
		metadata.Repo,
		metadata.Repo,
	)

	// Create relationship to repository
	err := m.relationshipService.CreateRelationship(
		ctx,
		models.NewEntityRelationship(
			models.RelationshipTypeContains,
			repoEntity,
			prEntity,
			models.DirectionOutgoing,
			1.0,
		).WithContext("Repository contains pull request"),
	)
	if err != nil {
		return fmt.Errorf("failed to create relationship to repository: %w", err)
	}

	return nil
}

// processCommitRelationships processes relationships for a commit
func (m *GitHubRelationshipManager) processCommitRelationships(
	ctx context.Context,
	commitEntity models.EntityID,
	metadata *storage.ContentMetadata,
	content []byte,
) error {
	// Extract commit data from metadata
	commitData, ok := metadata.GetMetadata()["commit"].(map[string]interface{})
	if !ok {
		return nil // No commit data available
	}

	// Process author information
	if author, ok := commitData["author"].(string); ok {
		// Create entity ID for author
		authorEntity := models.NewEntityID(
			models.EntityTypeUser,
			author,
			"",
			author,
		)

		// Create relationship
		err := m.relationshipService.CreateRelationship(
			ctx,
			models.NewEntityRelationship(
				models.RelationshipTypeCreates,
				authorEntity,
				commitEntity,
				models.DirectionOutgoing,
				1.0,
			).WithContext("User authored this commit"),
		)
		if err != nil {
			return fmt.Errorf("failed to create relationship to author: %w", err)
		}
	}

	// Process changed files
	if changedFiles, ok := commitData["changed_files"].([]interface{}); ok {
		for _, fileRef := range changedFiles {
			if filePath, ok := fileRef.(string); ok {
				// Create entity ID for file
				fileEntity := models.NewEntityID(
					models.EntityTypeFile,
					metadata.Owner,
					metadata.Repo,
					filePath,
				)

				// Create relationship
				err := m.relationshipService.CreateRelationship(
					ctx,
					models.NewEntityRelationship(
						models.RelationshipTypeModifies,
						commitEntity,
						fileEntity,
						models.DirectionOutgoing,
						0.9,
					).WithContext("Commit modifies this file"),
				)
				if err != nil {
					return fmt.Errorf("failed to create relationship to modified file: %w", err)
				}
			}
		}
	}

	// Process parent commits
	if parentCommits, ok := commitData["parents"].([]interface{}); ok {
		for _, parentRef := range parentCommits {
			if parentSha, ok := parentRef.(string); ok {
				// Create entity ID for parent commit
				parentEntity := models.NewEntityID(
					models.EntityTypeCommit,
					metadata.Owner,
					metadata.Repo,
					parentSha,
				)

				// Create relationship
				err := m.relationshipService.CreateRelationship(
					ctx,
					models.NewEntityRelationship(
						models.RelationshipTypeDependsOn,
						commitEntity,
						parentEntity,
						models.DirectionOutgoing,
						0.9,
					).WithContext("Commit depends on parent commit"),
				)
				if err != nil {
					return fmt.Errorf("failed to create relationship to parent commit: %w", err)
				}
			}
		}
	}

	// Process relationship to repository
	repoEntity := models.NewEntityID(
		models.EntityTypeRepository,
		metadata.Owner,
		metadata.Repo,
		metadata.Repo,
	)

	// Create relationship to repository
	err := m.relationshipService.CreateRelationship(
		ctx,
		models.NewEntityRelationship(
			models.RelationshipTypeContains,
			repoEntity,
			commitEntity,
			models.DirectionOutgoing,
			1.0,
		).WithContext("Repository contains commit"),
	)
	if err != nil {
		return fmt.Errorf("failed to create relationship to repository: %w", err)
	}

	// If commit message references issues or PRs, create relationships
	if message, ok := commitData["message"].(string); ok {
		// Extract issue references
		issueRefs := extractIssueReferences(message)
		for _, issueNum := range issueRefs {
			// Create entity ID for issue
			issueEntity := models.NewEntityID(
				models.EntityTypeIssue,
				metadata.Owner,
				metadata.Repo,
				issueNum,
			)

			// Create relationship
			err := m.relationshipService.CreateRelationship(
				ctx,
				models.NewEntityRelationship(
					models.RelationshipTypeReferences,
					commitEntity,
					issueEntity,
					models.DirectionOutgoing,
					0.7,
				).WithContext("Referenced in commit message"),
			)
			if err != nil {
				return fmt.Errorf("failed to create relationship to referenced issue: %w", err)
			}
		}

		// Extract PR references
		prRefs := extractPRReferences(message)
		for _, prNum := range prRefs {
			// Create entity ID for PR
			prEntity := models.NewEntityID(
				models.EntityTypePullRequest,
				metadata.Owner,
				metadata.Repo,
				prNum,
			)

			// Create relationship
			err := m.relationshipService.CreateRelationship(
				ctx,
				models.NewEntityRelationship(
					models.RelationshipTypeReferences,
					commitEntity,
					prEntity,
					models.DirectionOutgoing,
					0.7,
				).WithContext("Referenced in commit message"),
			)
			if err != nil {
				return fmt.Errorf("failed to create relationship to referenced PR: %w", err)
			}
		}
	}

	return nil
}

// processFileRelationships processes relationships for a file
func (m *GitHubRelationshipManager) processFileRelationships(
	ctx context.Context,
	fileEntity models.EntityID,
	metadata *storage.ContentMetadata,
	content []byte,
) error {
	// Process relationship to repository
	repoEntity := models.NewEntityID(
		models.EntityTypeRepository,
		metadata.Owner,
		metadata.Repo,
		metadata.Repo,
	)

	// Create relationship to repository
	err := m.relationshipService.CreateRelationship(
		ctx,
		models.NewEntityRelationship(
			models.RelationshipTypeContains,
			repoEntity,
			fileEntity,
			models.DirectionOutgoing,
			1.0,
		).WithContext("Repository contains file"),
	)
	if err != nil {
		return fmt.Errorf("failed to create relationship to repository: %w", err)
	}

	// Process file content if available
	if len(content) > 0 {
		// For code files, extract imports and references
		// Note: This is a simplified implementation and could be extended with language-specific parsing
		fileImports := extractImports(string(content), metadata.ContentID)
		for _, importPath := range fileImports {
			// Create entity ID for imported file
			importedFileEntity := models.NewEntityID(
				models.EntityTypeFile,
				metadata.Owner,
				metadata.Repo,
				importPath,
			)

			// Create relationship
			err := m.relationshipService.CreateRelationship(
				ctx,
				models.NewEntityRelationship(
					models.RelationshipTypeDependsOn,
					fileEntity,
					importedFileEntity,
					models.DirectionOutgoing,
					0.8,
				).WithContext("File imports this dependency"),
			)
			if err != nil {
				return fmt.Errorf("failed to create relationship to imported file: %w", err)
			}
		}
	}

	return nil
}

// processCommentRelationships processes relationships for a comment
func (m *GitHubRelationshipManager) processCommentRelationships(
	ctx context.Context,
	commentEntity models.EntityID,
	metadata *storage.ContentMetadata,
	content []byte,
) error {
	// Extract comment data from metadata
	commentData, ok := metadata.GetMetadata()["comment"].(map[string]interface{})
	if !ok {
		return nil // No comment data available
	}

	// Process parent entity (issue, PR, etc.)
	if parentType, ok := commentData["parent_type"].(string); ok {
		if parentID, ok := commentData["parent_id"].(string); ok {
			var entityType models.EntityType

			// Map parent type to entity type
			switch parentType {
			case "issue":
				entityType = models.EntityTypeIssue
			case "pull_request":
				entityType = models.EntityTypePullRequest
			case "commit":
				entityType = models.EntityTypeCommit
			default:
				entityType = models.EntityType(parentType)
			}

			// Create entity ID for parent
			parentEntity := models.NewEntityID(
				entityType,
				metadata.Owner,
				metadata.Repo,
				parentID,
			)

			// Create relationship
			err := m.relationshipService.CreateRelationship(
				ctx,
				models.NewEntityRelationship(
					models.RelationshipTypeComments,
					commentEntity,
					parentEntity,
					models.DirectionOutgoing,
					0.9,
				).WithContext("Comment on parent entity"),
			)
			if err != nil {
				return fmt.Errorf("failed to create relationship to parent entity: %w", err)
			}
		}
	}

	// Process author information
	if author, ok := commentData["author"].(string); ok {
		// Create entity ID for author
		authorEntity := models.NewEntityID(
			models.EntityTypeUser,
			author,
			"",
			author,
		)

		// Create relationship
		err := m.relationshipService.CreateRelationship(
			ctx,
			models.NewEntityRelationship(
				models.RelationshipTypeCreates,
				authorEntity,
				commentEntity,
				models.DirectionOutgoing,
				1.0,
			).WithContext("User authored this comment"),
		)
		if err != nil {
			return fmt.Errorf("failed to create relationship to author: %w", err)
		}
	}

	// Process mentions from comment text
	if len(content) > 0 {
		mentions := extractMentions(string(content))
		for _, username := range mentions {
			// Create entity ID for mentioned user
			userEntity := models.NewEntityID(
				models.EntityTypeUser,
				username,
				"",
				username,
			)

			// Create relationship
			err := m.relationshipService.CreateRelationship(
				ctx,
				models.NewEntityRelationship(
					models.RelationshipTypeReferences,
					commentEntity,
					userEntity,
					models.DirectionOutgoing,
					0.5,
				).WithContext("Mentioned in comment"),
			)
			if err != nil {
				return fmt.Errorf("failed to create relationship to mentioned user: %w", err)
			}
		}

		// Extract issue/PR references
		issueRefs := extractIssueReferences(string(content))
		for _, issueNum := range issueRefs {
			// Create entity ID for issue
			issueEntity := models.NewEntityID(
				models.EntityTypeIssue,
				metadata.Owner,
				metadata.Repo,
				issueNum,
			)

			// Create relationship
			err := m.relationshipService.CreateRelationship(
				ctx,
				models.NewEntityRelationship(
					models.RelationshipTypeReferences,
					commentEntity,
					issueEntity,
					models.DirectionOutgoing,
					0.6,
				).WithContext("Referenced in comment"),
			)
			if err != nil {
				return fmt.Errorf("failed to create relationship to referenced issue: %w", err)
			}
		}
	}

	return nil
}

// Helper functions for extracting references from text

// extractIssueReferences extracts issue numbers from text
func extractIssueReferences(text string) []string {
	// Match patterns like #123, issue #123, fixes #123, closes #123, etc.
	re := regexp.MustCompile(`(?i)(?:issue|fixes|closes|resolves|fix|close|resolve)?\s*#(\d+)`)
	matches := re.FindAllStringSubmatch(text, -1)

	var issueNums []string
	for _, match := range matches {
		if len(match) > 1 {
			issueNums = append(issueNums, match[1])
		}
	}

	return issueNums
}

// extractPRReferences extracts PR numbers from text
func extractPRReferences(text string) []string {
	// Match patterns like PR #123, pull request #123, etc.
	re := regexp.MustCompile(`(?i)(?:PR|pull request)\s*#(\d+)`)
	matches := re.FindAllStringSubmatch(text, -1)

	var prNums []string
	for _, match := range matches {
		if len(match) > 1 {
			prNums = append(prNums, match[1])
		}
	}

	return prNums
}

// extractFileReferences extracts file paths from text
func extractFileReferences(text string) []string {
	// Match file paths like path/to/file.go, src/file.js, etc.
	re := regexp.MustCompile(`\b(?:[\w-]+/)+[\w.-]+\b`)
	matches := re.FindAllString(text, -1)

	var filePaths []string
	for _, match := range matches {
		// Only include if looks like a file path (has an extension or directory structure)
		if strings.Contains(match, ".") || strings.Contains(match, "/") {
			filePaths = append(filePaths, match)
		}
	}

	return filePaths
}

// extractMentions extracts @username mentions from text
func extractMentions(text string) []string {
	// Match @username patterns
	re := regexp.MustCompile(`@([\w-]+)`)
	matches := re.FindAllStringSubmatch(text, -1)

	var usernames []string
	for _, match := range matches {
		if len(match) > 1 {
			usernames = append(usernames, match[1])
		}
	}

	return usernames
}

// extractImports extracts import paths from file content
func extractImports(content string, filePath string) []string {
	var imports []string

	// Determine file type from path
	if strings.HasSuffix(filePath, ".go") {
		// Go imports
		re := regexp.MustCompile(`import\s+\(([^)]+)\)`)
		importBlocks := re.FindAllStringSubmatch(content, -1)

		for _, block := range importBlocks {
			if len(block) > 1 {
				// Extract individual imports
				lines := strings.Split(block[1], "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "//") {
						// Remove quotes and extract package path
						line = strings.Trim(line, `" `)
						if line != "" {
							imports = append(imports, line)
						}
					}
				}
			}
		}
	} else if strings.HasSuffix(filePath, ".js") || strings.HasSuffix(filePath, ".ts") {
		// JavaScript/TypeScript imports
		re := regexp.MustCompile(`(?:import|require)\s*\(?['"]([^'"]+)['"]`)
		matches := re.FindAllStringSubmatch(content, -1)

		for _, match := range matches {
			if len(match) > 1 {
				imports = append(imports, match[1])
			}
		}
	} else if strings.HasSuffix(filePath, ".py") {
		// Python imports
		re := regexp.MustCompile(`(?:import|from)\s+([\w.]+)`)
		matches := re.FindAllStringSubmatch(content, -1)

		for _, match := range matches {
			if len(match) > 1 {
				imports = append(imports, match[1])
			}
		}
	}

	return imports
}
