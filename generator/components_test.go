package generator

import (
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupComponentsTest() *Generator {
	return &Generator{
		normalizer: &Normalizer{},
		typee: &Type{
			normalizer: &Normalizer{},
			config: &configurator.Config{
				ComponentsPackage: "components",
				Package:           "api",
			},
		},
		config: &configurator.Config{
			ComponentsPackage: "components",
			Package:           "api",
		},
		useRegex: make(map[string]string),
	}
}

func TestGenerator_components(t *testing.T) {
	generator := setupComponentsTest()
	
	swagger := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: map[string]*openapi3.SchemaRef{
				"User": {
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"object"},
						Properties: map[string]*openapi3.SchemaRef{
							"id": {
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"integer"},
								},
							},
							"name": {
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								},
							},
						},
						Required: []string{"id", "name"},
					},
				},
				"Product": {
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"object"},
						Properties: map[string]*openapi3.SchemaRef{
							"title": {
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								},
							},
							"price": {
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"number"},
								},
							},
						},
					},
				},
			},
		},
	}

	result := generator.components(swagger)
	require.NotNil(t, result)

	// Generate code and verify structure
	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "type User struct")
	assert.Contains(t, codeStr, "type Product struct")
	assert.Contains(t, codeStr, "ID   int")
	assert.Contains(t, codeStr, "Name string")
	assert.Contains(t, codeStr, "Title string")
	assert.Contains(t, codeStr, "Price float64")
}

func TestGenerator_componentFromSchema(t *testing.T) {
	generator := setupComponentsTest()

	tests := []struct {
		name           string
		componentName  string
		schema         *openapi3.SchemaRef
		expectedInCode []string
	}{
		{
			name:          "Simple object schema",
			componentName: "User",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id": {
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"integer"},
							},
						},
						"email": {
							Value: &openapi3.Schema{
								Type:   &openapi3.Types{"string"},
								Format: "email",
							},
						},
					},
					Required: []string{"id"},
				},
			},
			expectedInCode: []string{
				"type User struct",
				"ID    int",
				"Email string",
				`json:"id"`,
				`json:"email"`,
			},
		},
		{
			name:          "Schema with custom x-go-type",
			componentName: "CustomType",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
					Extensions: map[string]interface{}{
						"x-go-type": "time.Time",
					},
				},
			},
			expectedInCode: []string{
				"type CustomType = time.Time",
			},
		},
		{
			name:          "Schema with array property",
			componentName: "Container",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"items": {
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"array"},
								Items: &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: &openapi3.Types{"string"},
									},
								},
							},
						},
					},
				},
			},
			expectedInCode: []string{
				"type Container struct",
				"Items []string",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.componentFromSchema(tt.componentName, tt.schema)
			require.NotNil(t, result)

			file := jen.NewFile("test").Add(result)
		codeStr := file.GoString()
			for _, expected := range tt.expectedInCode {
				assert.Contains(t, codeStr, expected, "Expected %q in generated code", expected)
			}
		})
	}
}

func TestGenerator_enums(t *testing.T) {
	generator := setupComponentsTest()

	swagger := &openapi3.T{
		Components: &openapi3.Components{
			Schemas: map[string]*openapi3.SchemaRef{
				"Status": {
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"string"},
						Enum: []interface{}{"active", "inactive", "pending"},
					},
				},
				"Priority": {
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"integer"},
						Enum: []interface{}{1, 2, 3},
					},
				},
			},
		},
	}

	result := generator.enums(swagger)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	
	// Debug: Print the actual generated code
	t.Logf("Generated code: %q", codeStr)
	
	assert.Contains(t, codeStr, "type Status string")
	assert.Contains(t, codeStr, "type Priority int")
	assert.Contains(t, codeStr, `StatusActive   Status = "active"`)
	assert.Contains(t, codeStr, `StatusInactive Status = "inactive"`)
	assert.Contains(t, codeStr, `StatusPending  Status = "pending"`)
	assert.Contains(t, codeStr, "Priority1 Priority = 1")
	assert.Contains(t, codeStr, "Priority2 Priority = 2")
	assert.Contains(t, codeStr, "Priority3 Priority = 3")
}

func TestGenerator_enumFromSchema(t *testing.T) {
	generator := setupComponentsTest()

	tests := []struct {
		name           string
		enumName       string
		schema         *openapi3.SchemaRef
		expectedInCode []string
	}{
		{
			name:     "String enum",
			enumName: "Color",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
					Enum: []interface{}{"red", "green", "blue"},
				},
			},
			expectedInCode: []string{
				"type Color string",
				`ColorRed   Color = "red"`,
				`ColorGreen Color = "green"`,
				`ColorBlue  Color = "blue"`,
			},
		},
		{
			name:     "Integer enum",
			enumName: "Level",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"integer"},
					Enum: []interface{}{10, 20, 30},
				},
			},
			expectedInCode: []string{
				"type Level int",
				"Level10 Level = 10",
				"Level20 Level = 20",
				"Level30 Level = 30",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.enumFromSchema(tt.enumName, tt.schema)
			require.NotNil(t, result)

			file := jen.NewFile("test").Add(result)
			codeStr := file.GoString()
			for _, expected := range tt.expectedInCode {
				assert.Contains(t, codeStr, expected, "Expected %q in generated code", expected)
			}
		})
	}
}

func TestGenerator_generateEnumValidation(t *testing.T) {
	generator := setupComponentsTest()

	tests := []struct {
		name           string
		enumName       string
		schema         *openapi3.SchemaRef
		expectedInCode []string
	}{
		{
			name:     "String enum validation",
			enumName: "Status",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
					Enum: []interface{}{"active", "inactive"},
				},
			},
			expectedInCode: []string{
				"func (s Status) Validate() error",
				"switch s {",
				"case StatusActive:",
				"case StatusInactive:",
				"return nil",
				"default:",
				"return fmt.Errorf",
			},
		},
		{
			name:     "Integer enum validation",
			enumName: "Priority",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"integer"},
					Enum: []interface{}{1, 2, 3},
				},
			},
			expectedInCode: []string{
				"func (p Priority) Validate() error",
				"switch p {",
				"case Priority1:",
				"case Priority2:",
				"case Priority3:",
				"return nil",
				"default:",
				"return fmt.Errorf",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateEnumValidation(tt.enumName, tt.schema)
			require.NotNil(t, result)

			file := jen.NewFile("test").Add(result)
			codeStr := file.GoString()
			for _, expected := range tt.expectedInCode {
				assert.Contains(t, codeStr, expected, "Expected %q in generated code", expected)
			}
		})
	}
}

func TestGenerator_typeProperties(t *testing.T) {
	generator := setupComponentsTest()

	schema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"id": {
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"integer"},
				},
			},
			"name": {
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
			"email": {
				Value: &openapi3.Schema{
					Type:   &openapi3.Types{"string"},
					Format: "email",
				},
			},
			"optional_field": {
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
		},
		Required: []string{"id", "name"},
	}

	tests := []struct {
		name                 string
		typeName             string
		pointersForRequired  bool
		expectedFieldCount   int
		expectedRequiredTags []string
	}{
		{
			name:                "Without pointers for required",
			typeName:            "User",
			pointersForRequired: false,
			expectedFieldCount:  4,
			expectedRequiredTags: []string{
				"ID            int",
				"Name          string",
				"Email         string",
				"OptionalField string",
			},
		},
		{
			name:                "With pointers for required",
			typeName:            "User",
			pointersForRequired: true,
			expectedFieldCount:  4,
			expectedRequiredTags: []string{
				"ID            *int",    // Required fields get pointers in helper structs
				"Name          *string", // Required fields get pointers in helper structs
				"Email         string",  // Optional fields remain regular in helper structs
				"OptionalField string", // Optional fields remain regular in helper structs
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.typeProperties(tt.typeName, schema, tt.pointersForRequired)
			require.Len(t, result, tt.expectedFieldCount)

			// Combine all properties into a single code block for testing
			combined := jen.Type().Id("TestStruct").Struct(result...)
			codeStr := combined.GoString()

			for _, expected := range tt.expectedRequiredTags {
				assert.Contains(t, codeStr, expected, "Expected %q in generated properties", expected)
			}
		})
	}
}

func TestGenerator_components_EdgeCases(t *testing.T) {
	generator := setupComponentsTest()

	tests := []struct {
		name    string
		swagger *openapi3.T
		expect  func(t *testing.T, result jen.Code)
	}{
		{
			name: "Empty components",
			swagger: &openapi3.T{
				Components: &openapi3.Components{
					Schemas: map[string]*openapi3.SchemaRef{},
				},
			},
			expect: func(t *testing.T, result jen.Code) {
				// Should return empty/null code
				assert.NotNil(t, result)
			},
		},
		{
			name: "Nil components",
			swagger: &openapi3.T{
				Components: nil,
			},
			expect: func(t *testing.T, result jen.Code) {
				// Should handle nil gracefully
				assert.NotNil(t, result)
			},
		},
		{
			name: "Schema with extensions",
			swagger: &openapi3.T{
				Components: &openapi3.Components{
					Schemas: map[string]*openapi3.SchemaRef{
						"CustomType": {
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
								Extensions: map[string]interface{}{
									"x-go-type": "github.com/example/custom.Type",
								},
							},
						},
					},
				},
			},
			expect: func(t *testing.T, result jen.Code) {
				file := jen.NewFile("test").Add(result)
		codeStr := file.GoString()
				assert.Contains(t, codeStr, "custom.Type")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.components(tt.swagger)
			tt.expect(t, result)
		})
	}
}

func TestGenerator_componentFromSchema_WithExtensions(t *testing.T) {
	generator := setupComponentsTest()

	tests := []struct {
		name      string
		schema    *openapi3.SchemaRef
		expectErr bool
	}{
		{
			name: "Schema with x-go-omitempty",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"optional": {
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
								Extensions: map[string]interface{}{
									"x-go-omitempty": true,
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Schema with x-go-pointer",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"pointer_field": {
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
								Extensions: map[string]interface{}{
									"x-go-pointer": true,
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.componentFromSchema("TestType", tt.schema)
			if tt.expectErr {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				// Verify the code can be generated without panic
				file := jen.NewFile("test").Add(result)
		codeStr := file.GoString()
				assert.NotEmpty(t, codeStr)
			}
		})
	}
}