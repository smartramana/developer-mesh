package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaGenerator generates MCP-compatible tool schemas from OpenAPI specs
type SchemaGenerator struct {
	// Configuration for schema generation
	MaxOperationsPerTool int
	GroupByTag           bool
	IncludeDeprecated    bool

	// Operation grouper for multi-tool generation
	grouper *OperationGrouper
}

// NewSchemaGenerator creates a new schema generator with default settings
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		MaxOperationsPerTool: 50,   // Limit operations per tool to avoid overwhelming agents
		GroupByTag:           true, // Group operations by tag for better organization
		IncludeDeprecated:    false,
		grouper:              NewOperationGrouper(),
	}
}

// GenerateMCPSchema generates an MCP-compatible schema from an OpenAPI spec
// This returns a single unified schema that describes all available operations
func (g *SchemaGenerator) GenerateMCPSchema(spec *openapi3.T) (map[string]interface{}, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	// Build the MCP tool schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "The API operation to perform",
				"enum":        g.extractOperationIDs(spec),
			},
			"parameters": map[string]interface{}{
				"type":        "object",
				"description": "Parameters for the selected operation",
				"properties":  map[string]interface{}{},
			},
		},
		"required":             []string{"operation"},
		"additionalProperties": false,
	}

	// Add operation-specific parameter schemas
	operationSchemas := g.extractOperationSchemas(spec)
	if len(operationSchemas) > 0 {
		schema["allOf"] = []interface{}{
			map[string]interface{}{
				"if": map[string]interface{}{
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"const": "dynamic",
						},
					},
				},
				"then": map[string]interface{}{
					"properties": map[string]interface{}{
						"parameters": operationSchemas,
					},
				},
			},
		}
	}

	// Add metadata about available operations
	schema["x-operations"] = g.extractOperationMetadata(spec)

	return schema, nil
}

// GenerateOperationSchemas generates individual schemas for each operation
// This is useful when you want to expose each operation as a separate tool
func (g *SchemaGenerator) GenerateOperationSchemas(spec *openapi3.T) (map[string]interface{}, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	schemas := make(map[string]interface{})

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			// Generate operation ID if not present
			operationID := operation.OperationID
			if operationID == "" {
				operationID = g.generateOperationID(method, path)
			}

			// Generate schema for this operation
			opSchema := g.generateOperationSchema(operation, method, path)
			schemas[operationID] = opSchema
		}
	}

	return schemas, nil
}

// extractOperationIDs extracts all operation IDs from the spec
func (g *SchemaGenerator) extractOperationIDs(spec *openapi3.T) []string {
	var operationIDs []string
	seen := make(map[string]bool)

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			operationID := operation.OperationID
			if operationID == "" {
				operationID = g.generateOperationID(method, path)
			}

			if !seen[operationID] {
				operationIDs = append(operationIDs, operationID)
				seen[operationID] = true
			}

			// Stop if we've reached the max
			if len(operationIDs) >= g.MaxOperationsPerTool {
				break
			}
		}

		if len(operationIDs) >= g.MaxOperationsPerTool {
			break
		}
	}

	return operationIDs
}

// extractOperationMetadata extracts metadata about each operation
func (g *SchemaGenerator) extractOperationMetadata(spec *openapi3.T) map[string]interface{} {
	metadata := make(map[string]interface{})

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			operationID := operation.OperationID
			if operationID == "" {
				operationID = g.generateOperationID(method, path)
			}

			metadata[operationID] = map[string]interface{}{
				"method":      method,
				"path":        path,
				"summary":     operation.Summary,
				"description": operation.Description,
				"tags":        operation.Tags,
				"deprecated":  operation.Deprecated,
			}
		}
	}

	return metadata
}

// extractOperationSchemas creates a combined schema for all operations
func (g *SchemaGenerator) extractOperationSchemas(spec *openapi3.T) map[string]interface{} {
	properties := make(map[string]interface{})

	for _, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for _, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			// Extract parameters for this operation
			params := g.extractOperationParameters(operation, pathItem.Parameters)
			if len(params) > 0 {
				for name, schema := range params {
					// Merge parameters from different operations
					if existing, ok := properties[name]; ok {
						// If parameter exists, try to merge schemas intelligently
						properties[name] = g.mergeSchemas(existing, schema)
					} else {
						properties[name] = schema
					}
				}
			}
		}
	}

	return properties
}

// generateOperationSchema generates a schema for a single operation
func (g *SchemaGenerator) generateOperationSchema(operation *openapi3.Operation, method, path string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":        "object",
		"description": g.getOperationDescription(operation),
		"properties":  make(map[string]interface{}),
		"required":    []string{},
	}

	properties := schema["properties"].(map[string]interface{})
	required := []string{}

	// Add path parameters
	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.In == "path" {
			paramSchema := g.parameterToSchema(param.Value)
			properties[param.Value.Name] = paramSchema
			if param.Value.Required {
				required = append(required, param.Value.Name)
			}
		}
	}

	// Add query parameters
	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.In == "query" {
			paramSchema := g.parameterToSchema(param.Value)
			properties[param.Value.Name] = paramSchema
			if param.Value.Required {
				required = append(required, param.Value.Name)
			}
		}
	}

	// Add request body
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if jsonContent, ok := operation.RequestBody.Value.Content["application/json"]; ok {
			if jsonContent.Schema != nil && jsonContent.Schema.Value != nil {
				bodySchema := g.schemaToMCPSchema(jsonContent.Schema.Value)
				properties["body"] = bodySchema
				if operation.RequestBody.Value.Required {
					required = append(required, "body")
				}
			}
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// extractOperationParameters extracts parameters from an operation
func (g *SchemaGenerator) extractOperationParameters(operation *openapi3.Operation, globalParams openapi3.Parameters) map[string]interface{} {
	params := make(map[string]interface{})

	// Process global parameters first
	for _, param := range globalParams {
		if param.Value != nil {
			params[param.Value.Name] = g.parameterToSchema(param.Value)
		}
	}

	// Process operation-specific parameters (override globals)
	for _, param := range operation.Parameters {
		if param.Value != nil {
			params[param.Value.Name] = g.parameterToSchema(param.Value)
		}
	}

	// Process request body
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if jsonContent, ok := operation.RequestBody.Value.Content["application/json"]; ok {
			if jsonContent.Schema != nil && jsonContent.Schema.Value != nil {
				// For request body, we prefix parameters with "body_" to avoid conflicts
				if jsonContent.Schema.Value.Properties != nil {
					for name, prop := range jsonContent.Schema.Value.Properties {
						if prop.Value != nil {
							params["body_"+name] = g.schemaToMCPSchema(prop.Value)
						}
					}
				}
			}
		}
	}

	return params
}

// parameterToSchema converts an OpenAPI parameter to MCP schema
func (g *SchemaGenerator) parameterToSchema(param *openapi3.Parameter) map[string]interface{} {
	schema := map[string]interface{}{
		"description": param.Description,
	}

	if param.Schema != nil && param.Schema.Value != nil {
		// Copy type information
		schemaValue := param.Schema.Value
		if schemaValue.Type != nil {
			schema["type"] = g.getSchemaType(schemaValue)
		}
		if len(schemaValue.Enum) > 0 {
			schema["enum"] = schemaValue.Enum
		}
		if schemaValue.Default != nil {
			schema["default"] = schemaValue.Default
		}
		if schemaValue.Pattern != "" {
			schema["pattern"] = schemaValue.Pattern
		}
		// MinLength and MaxLength are uint64 in openapi3, not pointers
		if schemaValue.MinLength > 0 {
			schema["minLength"] = schemaValue.MinLength
		}
		if schemaValue.MaxLength != nil && *schemaValue.MaxLength > 0 {
			schema["maxLength"] = *schemaValue.MaxLength
		}
	}

	return schema
}

// schemaToMCPSchema converts an OpenAPI schema to MCP schema
func (g *SchemaGenerator) schemaToMCPSchema(schema *openapi3.Schema) map[string]interface{} {
	// Handle composition schemas (oneOf, allOf, anyOf) by simplifying them
	// Claude's API doesn't support these at the top level
	if len(schema.OneOf) > 0 {
		// For oneOf, use the first schema as a fallback
		if schema.OneOf[0].Value != nil {
			return g.schemaToMCPSchema(schema.OneOf[0].Value)
		}
	}
	if len(schema.AllOf) > 0 {
		// For allOf, merge all schemas
		merged := map[string]interface{}{
			"type":        "object",
			"description": schema.Description,
			"properties":  make(map[string]interface{}),
		}
		for _, subSchema := range schema.AllOf {
			if subSchema.Value != nil {
				subMCP := g.schemaToMCPSchema(subSchema.Value)
				if props, ok := subMCP["properties"].(map[string]interface{}); ok {
					mergedProps := merged["properties"].(map[string]interface{})
					for k, v := range props {
						mergedProps[k] = v
					}
				}
			}
		}
		return merged
	}
	if len(schema.AnyOf) > 0 {
		// For anyOf, use the first schema as a fallback
		if schema.AnyOf[0].Value != nil {
			return g.schemaToMCPSchema(schema.AnyOf[0].Value)
		}
	}

	mcpSchema := map[string]interface{}{
		"type":        g.getSchemaType(schema),
		"description": schema.Description,
	}

	// Handle arrays
	if g.getSchemaType(schema) == "array" && schema.Items != nil && schema.Items.Value != nil {
		mcpSchema["items"] = g.schemaToMCPSchema(schema.Items.Value)
	}

	// Handle objects
	if g.getSchemaType(schema) == "object" && schema.Properties != nil {
		properties := make(map[string]interface{})
		for name, prop := range schema.Properties {
			if prop.Value != nil {
				properties[name] = g.schemaToMCPSchema(prop.Value)
			}
		}
		mcpSchema["properties"] = properties

		if len(schema.Required) > 0 {
			mcpSchema["required"] = schema.Required
		}
	}

	// Add constraints
	if len(schema.Enum) > 0 {
		mcpSchema["enum"] = schema.Enum
	}
	if schema.Default != nil {
		mcpSchema["default"] = schema.Default
	}
	if schema.Pattern != "" {
		mcpSchema["pattern"] = schema.Pattern
	}

	return mcpSchema
}

// getSchemaType returns the type of an OpenAPI schema as a string
func (g *SchemaGenerator) getSchemaType(schema *openapi3.Schema) string {
	if schema.Type == nil {
		return "string" // default
	}

	// Type is a *openapi3.Types in OpenAPI 3.1
	if schema.Type.Is("string") {
		return "string"
	} else if schema.Type.Is("number") {
		return "number"
	} else if schema.Type.Is("integer") {
		return "integer"
	} else if schema.Type.Is("boolean") {
		return "boolean"
	} else if schema.Type.Is("array") {
		return "array"
	} else if schema.Type.Is("object") {
		return "object"
	}

	return "string" // default
}

// generateOperationID generates an operation ID from method and path
func (g *SchemaGenerator) generateOperationID(method, path string) string {
	// Clean the path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	cleanParts := []string{strings.ToLower(method)}

	for _, part := range parts {
		// Skip parameters and version indicators
		if strings.HasPrefix(part, "{") || part == "v1" || part == "v2" || part == "api" {
			continue
		}
		cleanParts = append(cleanParts, part)
	}

	return strings.Join(cleanParts, "_")
}

// getOperationDescription gets a description for an operation
func (g *SchemaGenerator) getOperationDescription(operation *openapi3.Operation) string {
	if operation.Summary != "" {
		return operation.Summary
	}
	if operation.Description != "" {
		// Truncate long descriptions
		if len(operation.Description) > 200 {
			return operation.Description[:197] + "..."
		}
		return operation.Description
	}
	return "API operation"
}

// mergeSchemas attempts to merge two schemas intelligently
func (g *SchemaGenerator) mergeSchemas(existing, new interface{}) interface{} {
	// For now, just return the existing schema
	// In a more sophisticated implementation, we could merge enums, types, etc.
	return existing
}

// GenerateGroupedSchemas generates schemas for operation groups
// This is the main method for creating multiple tools from an OpenAPI spec
func (g *SchemaGenerator) GenerateGroupedSchemas(spec *openapi3.T) (map[string]GroupedToolSchema, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	// Group operations using the grouper
	groups, err := g.grouper.GroupOperations(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to group operations: %w", err)
	}

	// Generate schemas for each group
	schemas := make(map[string]GroupedToolSchema)

	for groupName, group := range groups {
		schema := g.generateGroupSchema(group)
		schemas[groupName] = GroupedToolSchema{
			Name:        groupName,
			DisplayName: group.DisplayName,
			Description: group.Description,
			Schema:      schema,
			Operations:  g.extractGroupOperationInfo(group),
			Priority:    group.Priority,
		}
	}

	return schemas, nil
}

// GroupedToolSchema represents a schema for a grouped tool
type GroupedToolSchema struct {
	Name        string                 // Tool name (e.g., "github_repos")
	DisplayName string                 // Human-friendly name
	Description string                 // Tool description
	Schema      map[string]interface{} // MCP-compatible schema
	Operations  []OperationInfo        // Information about operations
	Priority    int                    // Priority for ordering
}

// OperationInfo contains metadata about an operation
type OperationInfo struct {
	ID          string
	Method      string
	Path        string
	Summary     string
	Description string
}

// generateGroupSchema generates a schema for an operation group
func (g *SchemaGenerator) generateGroupSchema(group *OperationGroup) map[string]interface{} {
	// Collect all unique parameters from all operations in the group
	allParameters := make(map[string]interface{})
	operationParams := make(map[string][]string) // Track which params belong to which operation

	// Extract parameters from each operation
	for opID, op := range group.Operations {
		opSchema := g.generateOperationSchema(op.Operation, op.Method, op.Path)
		if props, ok := opSchema["properties"].(map[string]interface{}); ok {
			operationParams[opID] = make([]string, 0)
			for paramName, paramSchema := range props {
				// Add operation info to parameter description
				if paramDesc, ok := paramSchema.(map[string]interface{}); ok {
					if desc, hasDesc := paramDesc["description"].(string); hasDesc {
						paramDesc["description"] = fmt.Sprintf("[%s] %s", opID, desc)
					} else {
						paramDesc["description"] = fmt.Sprintf("Parameter for %s operation", opID)
					}
				}
				allParameters[paramName] = paramSchema
				operationParams[opID] = append(operationParams[opID], paramName)
			}
		}
	}

	// Build the MCP tool schema for this group
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": fmt.Sprintf("The %s operation to perform", group.DisplayName),
				"enum":        g.extractGroupOperationIDs(group),
			},
			"parameters": map[string]interface{}{
				"type":                 "object",
				"description":          "Parameters for the selected operation",
				"properties":           allParameters,
				"additionalProperties": false,
			},
		},
		"required":             []string{"operation"},
		"additionalProperties": false,
	}

	// Add metadata about operations and their parameters
	schema["x-operations"] = g.extractGroupOperationMetadata(group)
	schema["x-operation-params"] = operationParams

	return schema
}

// extractGroupOperationIDs extracts operation IDs from a group
func (g *SchemaGenerator) extractGroupOperationIDs(group *OperationGroup) []string {
	ids := make([]string, 0, len(group.Operations))
	for id := range group.Operations {
		ids = append(ids, id)
	}
	// Sort for consistency
	sort.Strings(ids)
	return ids
}

// extractGroupOperationSchemas extracts operation schemas for a group
func (g *SchemaGenerator) extractGroupOperationSchemas(group *OperationGroup) map[string]map[string]interface{} {
	schemas := make(map[string]map[string]interface{})

	for opID, op := range group.Operations {
		opSchema := g.generateOperationSchema(op.Operation, op.Method, op.Path)
		schemas[opID] = opSchema
	}

	return schemas
}

// extractGroupOperationMetadata extracts metadata for operations in a group
func (g *SchemaGenerator) extractGroupOperationMetadata(group *OperationGroup) map[string]interface{} {
	metadata := make(map[string]interface{})

	for opID, op := range group.Operations {
		metadata[opID] = map[string]interface{}{
			"method":      op.Method,
			"path":        op.Path,
			"summary":     op.Operation.Summary,
			"description": op.Operation.Description,
			"tags":        op.Operation.Tags,
			"deprecated":  op.Operation.Deprecated,
		}
	}

	return metadata
}

// extractGroupOperationInfo extracts operation information for documentation
func (g *SchemaGenerator) extractGroupOperationInfo(group *OperationGroup) []OperationInfo {
	info := make([]OperationInfo, 0, len(group.Operations))

	for opID, op := range group.Operations {
		info = append(info, OperationInfo{
			ID:          opID,
			Method:      op.Method,
			Path:        op.Path,
			Summary:     op.Operation.Summary,
			Description: op.Operation.Description,
		})
	}

	// Sort by operation ID for consistency
	sort.Slice(info, func(i, j int) bool {
		return info[i].ID < info[j].ID
	})

	return info
}

// ConfigureGrouping configures the operation grouping strategy
func (g *SchemaGenerator) ConfigureGrouping(strategy GroupingStrategy, maxPerGroup int) {
	if g.grouper != nil {
		g.grouper.GroupingStrategy = strategy
		g.grouper.MaxOperationsPerGroup = maxPerGroup
	}
}
