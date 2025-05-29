// Package tool provides tool infrastructure for the MCP application
package tool

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ToolDefinition represents a tool that can be used by an AI model
type ToolDefinition struct {
	// Name is the unique identifier for this tool
	Name string `json:"name"`

	// Description is a human-readable description of the tool
	Description string `json:"description"`

	// Parameters defines the schema for the tool's parameters
	Parameters ParameterSchema `json:"parameters"`

	// Returns defines the schema for the tool's return value
	Returns ReturnSchema `json:"returns,omitempty"`

	// Examples provides usage examples for the tool
	Examples []ToolExample `json:"examples,omitempty"`

	// Tags for categorizing and filtering tools
	Tags []string `json:"tags,omitempty"`

	// IsAsync indicates if the tool is asynchronous
	IsAsync bool `json:"is_async,omitempty"`
}

// ParameterSchema defines the schema for a tool's parameters
type ParameterSchema struct {
	// Type is the schema type (usually "object")
	Type string `json:"type"`

	// Properties defines the parameters as property schemas
	Properties map[string]PropertySchema `json:"properties"`

	// Required lists the names of required parameters
	Required []string `json:"required,omitempty"`
}

// PropertySchema defines the schema for a parameter property
type PropertySchema struct {
	// Type is the data type of the property
	Type string `json:"type"`

	// Description is a human-readable description of the property
	Description string `json:"description"`

	// Enum lists the possible values for this property (if applicable)
	Enum []string `json:"enum,omitempty"`

	// Default is the default value for this property (if applicable)
	Default interface{} `json:"default,omitempty"`

	// Items defines the schema for array items (if type is "array")
	Items *PropertySchema `json:"items,omitempty"`

	// Properties defines the schema for object properties (if type is "object")
	Properties map[string]PropertySchema `json:"properties,omitempty"`
}

// ReturnSchema defines the schema for a tool's return value
type ReturnSchema struct {
	// Type is the data type of the return value
	Type string `json:"type"`

	// Description is a human-readable description of the return value
	Description string `json:"description"`

	// Properties defines the schema for object properties (if type is "object")
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Items defines the schema for array items (if type is "array")
	Items *PropertySchema `json:"items,omitempty"`
}

// ToolExample provides a usage example for a tool
type ToolExample struct {
	// Description explains what this example demonstrates
	Description string `json:"description"`

	// Parameters show sample parameter values
	Parameters map[string]interface{} `json:"parameters"`

	// Result shows the expected return value
	Result interface{} `json:"result,omitempty"`
}

// ToolHandler is a function that handles a tool call
type ToolHandler func(params map[string]interface{}) (interface{}, error)

// Tool represents a complete tool with definition and handler
type Tool struct {
	Definition ToolDefinition
	Handler    ToolHandler
}

// ToolRegistry holds all available tools
type ToolRegistry struct {
	tools map[string]*Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

// RegisterTool registers a tool with the registry
func (r *ToolRegistry) RegisterTool(tool *Tool) error {
	if _, exists := r.tools[tool.Definition.Name]; exists {
		return fmt.Errorf("tool '%s' is already registered", tool.Definition.Name)
	}
	r.tools[tool.Definition.Name] = tool
	return nil
}

// GetTool returns a tool by name
func (r *ToolRegistry) GetTool(name string) (*Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}
	return tool, nil
}

// ListTools returns all registered tools
func (r *ToolRegistry) ListTools() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ValidateParams validates tool parameters against the tool's schema
func (t *Tool) ValidateParams(params map[string]interface{}) error {
	// Check required parameters
	for _, required := range t.Definition.Parameters.Required {
		if _, exists := params[required]; !exists {
			return fmt.Errorf("missing required parameter: %s", required)
		}
	}

	// Validate parameter types and values
	for name, schema := range t.Definition.Parameters.Properties {
		value, exists := params[name]
		if !exists {
			continue // Parameter is not provided (and not required)
		}

		if err := validateProperty(name, value, schema); err != nil {
			return err
		}
	}

	return nil
}

// validateProperty validates a property value against its schema
func validateProperty(name string, value interface{}, schema PropertySchema) error {
	if value == nil {
		return nil // Allow nil values for now
	}

	valueType := reflect.TypeOf(value)
	
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter '%s' must be a string", name)
		}
		
		// Validate enum if specified
		if len(schema.Enum) > 0 {
			strValue := value.(string)
			valid := false
			for _, enumValue := range schema.Enum {
				if strValue == enumValue {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("parameter '%s' must be one of: %s", name, strings.Join(schema.Enum, ", "))
			}
		}
		
	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64:
			// These are acceptable number types
		default:
			return fmt.Errorf("parameter '%s' must be a number", name)
		}
		
	case "integer":
		switch value.(type) {
		case int, int32, int64:
			// These are acceptable integer types
		default:
			return fmt.Errorf("parameter '%s' must be an integer", name)
		}
		
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter '%s' must be a boolean", name)
		}
		
	case "array":
		if valueType.Kind() != reflect.Slice && valueType.Kind() != reflect.Array {
			return fmt.Errorf("parameter '%s' must be an array", name)
		}
		
		// Validate array items if schema is provided
		if schema.Items != nil {
			arrayValue := reflect.ValueOf(value)
			for i := 0; i < arrayValue.Len(); i++ {
				item := arrayValue.Index(i).Interface()
				if err := validateProperty(fmt.Sprintf("%s[%d]", name, i), item, *schema.Items); err != nil {
					return err
				}
			}
		}
		
	case "object":
		if valueType.Kind() != reflect.Map {
			return fmt.Errorf("parameter '%s' must be an object", name)
		}
		
		// Validate object properties if schema is provided
		if len(schema.Properties) > 0 {
			mapValue, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("parameter '%s' must be a map[string]interface{}", name)
			}
			
			for propName, propSchema := range schema.Properties {
				propValue, exists := mapValue[propName]
				if !exists {
					continue // Property is not provided
				}
				
				if err := validateProperty(fmt.Sprintf("%s.%s", name, propName), propValue, propSchema); err != nil {
					return err
				}
			}
		}
	}
	
	return nil
}

// ToJSON converts a tool definition to JSON
func (t *ToolDefinition) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// FromJSON parses a tool definition from JSON
func ToolDefinitionFromJSON(data []byte) (*ToolDefinition, error) {
	var def ToolDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	return &def, nil
}
