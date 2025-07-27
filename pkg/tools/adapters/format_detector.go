package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// APIFormat represents different API documentation formats
type APIFormat string

const (
	FormatOpenAPI3     APIFormat = "openapi3"
	FormatOpenAPI2     APIFormat = "openapi2"
	FormatSwagger      APIFormat = "swagger"
	FormatRAML         APIFormat = "raml"
	FormatAPIBlueprint APIFormat = "apiblueprint"
	FormatPostman      APIFormat = "postman"
	FormatInsomnia     APIFormat = "insomnia"
	FormatCustomJSON   APIFormat = "custom_json"
	FormatGraphQL      APIFormat = "graphql"
	FormatGRPC         APIFormat = "grpc"
	FormatUnknown      APIFormat = "unknown"
)

// FormatDetector detects and converts various API documentation formats
type FormatDetector struct {
	httpClient *http.Client
}

// NewFormatDetector creates a new format detector
func NewFormatDetector(httpClient *http.Client) *FormatDetector {
	return &FormatDetector{
		httpClient: httpClient,
	}
}

// DetectFormat detects the format of API documentation
func (d *FormatDetector) DetectFormat(content []byte) (APIFormat, error) {
	// Try to parse as JSON first
	var jsonData map[string]interface{}
	if err := json.Unmarshal(content, &jsonData); err == nil {
		return d.detectJSONFormat(jsonData)
	}

	// Try YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(content, &yamlData); err == nil {
		return d.detectYAMLFormat(yamlData)
	}

	// Check for other text-based formats
	contentStr := string(content)
	if strings.Contains(contentStr, "#%RAML") {
		return FormatRAML, nil
	}
	if strings.Contains(contentStr, "FORMAT:") && strings.Contains(contentStr, "HOST:") {
		return FormatAPIBlueprint, nil
	}
	if strings.Contains(contentStr, "type Query") || strings.Contains(contentStr, "schema {") {
		return FormatGraphQL, nil
	}

	return FormatUnknown, nil
}

// detectJSONFormat detects format from parsed JSON
func (d *FormatDetector) detectJSONFormat(data map[string]interface{}) (APIFormat, error) {
	// OpenAPI 3.x
	if openapi, ok := data["openapi"].(string); ok && strings.HasPrefix(openapi, "3.") {
		return FormatOpenAPI3, nil
	}

	// Swagger 2.0 / OpenAPI 2.0
	if swagger, ok := data["swagger"].(string); ok && swagger == "2.0" {
		return FormatSwagger, nil
	}

	// Postman Collection
	if info, ok := data["info"].(map[string]interface{}); ok {
		if schema, ok := info["schema"].(string); ok && strings.Contains(schema, "postman") {
			return FormatPostman, nil
		}
		if _, ok := info["_postman_id"]; ok {
			return FormatPostman, nil
		}
	}

	// Insomnia
	if _, ok := data["_type"].(string); ok {
		if strings.Contains(fmt.Sprint(data["_type"]), "export") {
			return FormatInsomnia, nil
		}
	}

	// Custom JSON API (like SonarQube)
	if d.isCustomAPIFormat(data) {
		return FormatCustomJSON, nil
	}

	return FormatUnknown, nil
}

// detectYAMLFormat detects format from parsed YAML
func (d *FormatDetector) detectYAMLFormat(data map[string]interface{}) (APIFormat, error) {
	// Similar checks as JSON
	return d.detectJSONFormat(data)
}

// isCustomAPIFormat checks if it's a custom API format
func (d *FormatDetector) isCustomAPIFormat(data map[string]interface{}) bool {
	// SonarQube-style API listing
	if webServices, ok := data["webServices"].([]interface{}); ok && len(webServices) > 0 {
		return true
	}

	// Generic API listing patterns
	if apis, ok := data["apis"].([]interface{}); ok && len(apis) > 0 {
		return true
	}
	if endpoints, ok := data["endpoints"].([]interface{}); ok && len(endpoints) > 0 {
		return true
	}
	if services, ok := data["services"].([]interface{}); ok && len(services) > 0 {
		return true
	}

	return false
}

// ConvertToOpenAPI attempts to convert various formats to OpenAPI 3.0
func (d *FormatDetector) ConvertToOpenAPI(content []byte, format APIFormat, baseURL string) (*openapi3.T, error) {
	switch format {
	case FormatOpenAPI3:
		// Already in the right format
		loader := openapi3.NewLoader()
		return loader.LoadFromData(content)

	case FormatSwagger, FormatOpenAPI2:
		// Convert Swagger 2.0 to OpenAPI 3.0
		return d.convertSwagger2ToOpenAPI3(content)

	case FormatCustomJSON:
		// Convert custom JSON formats
		return d.convertCustomJSONToOpenAPI(content, baseURL)

	case FormatPostman:
		// Convert Postman collection
		return d.convertPostmanToOpenAPI(content, baseURL)

	default:
		return nil, fmt.Errorf("unsupported format for conversion: %s", format)
	}
}

// convertSwagger2ToOpenAPI3 converts Swagger 2.0 to OpenAPI 3.0
func (d *FormatDetector) convertSwagger2ToOpenAPI3(content []byte) (*openapi3.T, error) {
	// The kin-openapi library can handle both Swagger 2.0 and OpenAPI 3.0
	loader := openapi3.NewLoader()
	return loader.LoadFromData(content)
}

// convertCustomJSONToOpenAPI converts custom JSON API formats to OpenAPI
func (d *FormatDetector) convertCustomJSONToOpenAPI(content []byte, baseURL string) (*openapi3.T, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	// Create a minimal OpenAPI spec
	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   "Auto-discovered API",
			Version: "1.0.0",
		},
		Servers: openapi3.Servers{
			&openapi3.Server{
				URL: baseURL,
			},
		},
		Paths: openapi3.NewPaths(),
	}

	// Handle SonarQube-style webServices
	if webServices, ok := data["webServices"].([]interface{}); ok {
		for _, service := range webServices {
			if svc, ok := service.(map[string]interface{}); ok {
				d.addServiceToOpenAPI(spec, svc)
			}
		}
	}

	// Handle generic API listings
	if apis, ok := data["apis"].([]interface{}); ok {
		for _, api := range apis {
			if apiMap, ok := api.(map[string]interface{}); ok {
				d.addAPIToOpenAPI(spec, apiMap)
			}
		}
	}

	return spec, nil
}

// addServiceToOpenAPI adds a service definition to OpenAPI spec
func (d *FormatDetector) addServiceToOpenAPI(spec *openapi3.T, service map[string]interface{}) {
	path := "/"
	if p, ok := service["path"].(string); ok {
		path = p
	}

	// Extract actions/operations
	if actions, ok := service["actions"].([]interface{}); ok {
		for _, action := range actions {
			if act, ok := action.(map[string]interface{}); ok {
				d.addActionToPath(spec, path, act)
			}
		}
	}
}

// addAPIToOpenAPI adds an API definition to OpenAPI spec
func (d *FormatDetector) addAPIToOpenAPI(spec *openapi3.T, api map[string]interface{}) {
	path := "/"
	if p, ok := api["path"].(string); ok {
		path = p
	} else if p, ok := api["endpoint"].(string); ok {
		path = p
	}

	method := "GET"
	if m, ok := api["method"].(string); ok {
		method = strings.ToLower(m)
	}

	desc := "Successful response"
	responses := openapi3.NewResponses()
	responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: &desc,
		},
	})
	operation := &openapi3.Operation{
		Summary:   fmt.Sprint(api["name"]),
		Responses: responses,
	}

	if spec.Paths.Find(path) == nil {
		spec.Paths.Set(path, &openapi3.PathItem{})
	}

	pathItem := spec.Paths.Find(path)
	switch method {
	case "get":
		pathItem.Get = operation
	case "post":
		pathItem.Post = operation
	case "put":
		pathItem.Put = operation
	case "delete":
		pathItem.Delete = operation
	case "patch":
		pathItem.Patch = operation
	}
}

// addActionToPath adds an action to a path in the OpenAPI spec
func (d *FormatDetector) addActionToPath(spec *openapi3.T, basePath string, action map[string]interface{}) {
	key := fmt.Sprint(action["key"])
	fullPath := basePath + "/" + key

	desc := "Successful response"
	responses := openapi3.NewResponses()
	responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: &desc,
		},
	})
	operation := &openapi3.Operation{
		Summary:     fmt.Sprint(action["description"]),
		OperationID: key,
		Responses:   responses,
	}

	// Add parameters if available
	if params, ok := action["params"].([]interface{}); ok {
		for _, param := range params {
			if p, ok := param.(map[string]interface{}); ok {
				operation.Parameters = append(operation.Parameters, d.createParameter(p))
			}
		}
	}

	if spec.Paths.Find(fullPath) == nil {
		spec.Paths.Set(fullPath, &openapi3.PathItem{})
	}

	// Default to POST for actions
	spec.Paths.Find(fullPath).Post = operation
}

// createParameter creates an OpenAPI parameter from custom format
func (d *FormatDetector) createParameter(param map[string]interface{}) *openapi3.ParameterRef {
	name := fmt.Sprint(param["key"])
	required := false
	if r, ok := param["required"].(bool); ok {
		required = r
	}

	return &openapi3.ParameterRef{
		Value: &openapi3.Parameter{
			Name:     name,
			In:       "query",
			Required: required,
			Schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
		},
	}
}

// convertPostmanToOpenAPI converts Postman collection to OpenAPI
func (d *FormatDetector) convertPostmanToOpenAPI(content []byte, baseURL string) (*openapi3.T, error) {
	// This would require a more complex conversion
	// For now, return an error indicating manual conversion needed
	return nil, fmt.Errorf("postman collection conversion not yet implemented")
}

// TryMultipleFormats attempts to fetch and parse API documentation in multiple formats
func (d *FormatDetector) TryMultipleFormats(ctx context.Context, baseURL string, paths []string) (*openapi3.T, APIFormat, error) {
	for _, path := range paths {
		url := strings.TrimRight(baseURL, "/") + path

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		resp, err := d.httpClient.Do(req)
		if err != nil {
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		content, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		format, err := d.DetectFormat(content)
		if err != nil {
			continue
		}

		if format != FormatUnknown {
			spec, err := d.ConvertToOpenAPI(content, format, baseURL)
			if err == nil {
				return spec, format, nil
			}
		}
	}

	return nil, FormatUnknown, fmt.Errorf("no valid API documentation found")
}
