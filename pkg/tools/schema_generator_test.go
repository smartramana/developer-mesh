package tools

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaGenerator_GenerateMCPSchema(t *testing.T) {
	tests := []struct {
		name    string
		spec    *openapi3.T
		wantErr bool
		check   func(t *testing.T, schema map[string]interface{})
	}{
		{
			name:    "nil spec returns error",
			spec:    nil,
			wantErr: true,
		},
		{
			name: "simple spec with one operation",
			spec: &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(
					openapi3.WithPath("/users", &openapi3.PathItem{
						Get: &openapi3.Operation{
							OperationID: "getUsers",
							Summary:     "Get all users",
						},
					}),
				),
			},
			wantErr: false,
			check: func(t *testing.T, schema map[string]interface{}) {
				assert.NotNil(t, schema)
				assert.Equal(t, "object", schema["type"])
				
				// Check that properties exist
				props, ok := schema["properties"].(map[string]interface{})
				require.True(t, ok, "properties should be a map")
				
				// Check operation field
				opField, ok := props["operation"].(map[string]interface{})
				require.True(t, ok, "operation field should exist")
				assert.Equal(t, "string", opField["type"])
				
				// Check that operation enum contains our operation
				enum, ok := opField["enum"].([]string)
				if ok && len(enum) > 0 {
					assert.Contains(t, enum, "getUsers")
				}
			},
		},
		{
			name: "spec with parameters",
			spec: &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(
					openapi3.WithPath("/users/{id}", &openapi3.PathItem{
						Get: &openapi3.Operation{
							OperationID: "getUser",
							Summary:     "Get user by ID",
							Parameters: openapi3.Parameters{
								{
									Value: &openapi3.Parameter{
										Name:     "id",
										In:       "path",
										Required: true,
										Schema: &openapi3.SchemaRef{
											Value: &openapi3.Schema{
												Type: &openapi3.Types{"string"},
											},
										},
									},
								},
							},
						},
					}),
				),
			},
			wantErr: false,
			check: func(t *testing.T, schema map[string]interface{}) {
				assert.NotNil(t, schema)
				
				// Check x-operations metadata
				if xOps, ok := schema["x-operations"].(map[string]interface{}); ok {
					if getUserOp, ok := xOps["getUser"].(map[string]interface{}); ok {
						assert.Equal(t, "GET", getUserOp["method"])
						assert.Equal(t, "/users/{id}", getUserOp["path"])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewSchemaGenerator()
			schema, err := g.GenerateMCPSchema(tt.spec)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
			if tt.check != nil {
				tt.check(t, schema)
			}
		})
	}
}

func TestSchemaGenerator_GenerateOperationSchemas(t *testing.T) {
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(
			openapi3.WithPath("/users", &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "listUsers",
					Summary:     "List all users",
				},
				Post: &openapi3.Operation{
					OperationID: "createUser",
					Summary:     "Create a new user",
					RequestBody: &openapi3.RequestBodyRef{
						Value: &openapi3.RequestBody{
							Required: true,
							Content: openapi3.Content{
								"application/json": &openapi3.MediaType{
									Schema: &openapi3.SchemaRef{
										Value: &openapi3.Schema{
											Type: &openapi3.Types{"object"},
											Properties: openapi3.Schemas{
												"name": &openapi3.SchemaRef{
													Value: &openapi3.Schema{
														Type: &openapi3.Types{"string"},
													},
												},
												"email": &openapi3.SchemaRef{
													Value: &openapi3.Schema{
														Type: &openapi3.Types{"string"},
													},
												},
											},
											Required: []string{"name", "email"},
										},
									},
								},
							},
						},
					},
				},
			}),
		),
	}

	g := NewSchemaGenerator()
	schemas, err := g.GenerateOperationSchemas(spec)
	
	assert.NoError(t, err)
	assert.NotNil(t, schemas)
	assert.Len(t, schemas, 2)
	
	// Check listUsers schema
	if listSchema, ok := schemas["listUsers"].(map[string]interface{}); ok {
		assert.Equal(t, "object", listSchema["type"])
		assert.Contains(t, listSchema["description"], "List all users")
	}
	
	// Check createUser schema
	if createSchema, ok := schemas["createUser"].(map[string]interface{}); ok {
		assert.Equal(t, "object", createSchema["type"])
		props, ok := createSchema["properties"].(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, props, "body")
	}
}