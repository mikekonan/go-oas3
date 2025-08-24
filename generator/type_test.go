package generator

import (
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTypeTest() *Type {
	return &Type{
		normalizer: &Normalizer{},
		config: &configurator.Config{
			ComponentsPackage: "components",
			Package:           "api",
		},
	}
}

func TestType_fillJsonTag(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name           string
		schemaRef      *openapi3.SchemaRef
		fieldName      string
		expectedTag    string
		expectedOmit   bool
	}{
		{
			name: "Simple field without omitempty",
			schemaRef: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
			fieldName:   "username",
			expectedTag: "username",
		},
		{
			name: "Field with omitempty extension",
			schemaRef: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
					Extensions: map[string]interface{}{
						"x-go-omitempty": true,
					},
				},
			},
			fieldName:   "optionalField",
			expectedTag: "optionalField,omitempty",
		},
		{
			name: "Uppercase field name",
			schemaRef: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
			fieldName:   "USERNAME",
			expectedTag: "username",
		},
		{
			name: "CamelCase field name",
			schemaRef: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
			fieldName:   "FirstName",
			expectedTag: "firstName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := jen.Id("TestField").Add(jen.String())
			typ.fillJsonTag(stmt, tt.schemaRef, tt.fieldName)
			
			// Create a proper struct field statement
			structStmt := jen.Type().Id("TestStruct").Struct(stmt)
			
			// Render the struct to check the tag
			code := structStmt.GoString()
			assert.Contains(t, code, tt.expectedTag)
		})
	}
}

func TestType_getXGoType(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name        string
		schema      *openapi3.Schema
		expectedPkg string
		expectedTyp string
		expectedOk  bool
	}{
		{
			name: "Custom type with package",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-type": "github.com/google/uuid.UUID",
				},
			},
			expectedPkg: "github.com/google/uuid",
			expectedTyp: "UUID",
			expectedOk:  true,
		},
		{
			name: "Custom type without package",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-type": "CustomType",
				},
			},
			expectedPkg: "",
			expectedTyp: "CustomType",
			expectedOk:  true,
		},
		{
			name: "No custom type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
			expectedPkg: "",
			expectedTyp: "",
			expectedOk:  false,
		},
		{
			name: "Empty extension",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{},
			},
			expectedPkg: "",
			expectedTyp: "",
			expectedOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, typeName, ok := typ.getXGoType(tt.schema)
			assert.Equal(t, tt.expectedPkg, pkg)
			assert.Equal(t, tt.expectedTyp, typeName)
			assert.Equal(t, tt.expectedOk, ok)
		})
	}
}

func TestType_getXGoPointer(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name     string
		schema   *openapi3.Schema
		expected bool
	}{
		{
			name: "Pointer extension true",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-pointer": true,
				},
			},
			expected: true,
		},
		{
			name: "Pointer extension false",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-pointer": false,
				},
			},
			expected: false,
		},
		{
			name: "No pointer extension",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typ.getXGoPointer(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestType_getXGoOmitempty(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name     string
		schema   *openapi3.Schema
		expected bool
	}{
		{
			name: "Omitempty extension true",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-omitempty": true,
				},
			},
			expected: true,
		},
		{
			name: "Omitempty extension false",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-omitempty": false,
				},
			},
			expected: false,
		},
		{
			name: "No omitempty extension",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typ.getXGoOmitempty(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestType_getXGoSkipValidation(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name     string
		schema   *openapi3.Schema
		expected bool
	}{
		{
			name: "Skip validation true",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-skip-validation": true,
				},
			},
			expected: true,
		},
		{
			name: "Skip validation false",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-skip-validation": false,
				},
			},
			expected: false,
		},
		{
			name: "No skip validation extension",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typ.getXGoSkipValidation(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestType_getXGoSkipSecurityCheck(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name      string
		operation *openapi3.Operation
		expected  bool
	}{
		{
			name: "Skip security check true",
			operation: &openapi3.Operation{
				Extensions: map[string]interface{}{
					"x-go-skip-security-check": true,
				},
			},
			expected: true,
		},
		{
			name: "Skip security check false",
			operation: &openapi3.Operation{
				Extensions: map[string]interface{}{
					"x-go-skip-security-check": false,
				},
			},
			expected: false,
		},
		{
			name: "No skip security check extension",
			operation: &openapi3.Operation{
				OperationID: "testOperation",
			},
			expected: false,
		},
		{
			name:      "Nil operation",
			operation: nil,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typ.getXGoSkipSecurityCheck(tt.operation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestType_isCustomType(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name     string
		schema   *openapi3.Schema
		expected bool
	}{
		{
			name: "String with format",
			schema: &openapi3.Schema{
				Type:   &openapi3.Types{"string"},
				Format: "email",
			},
			expected: true,
		},
		{
			name: "String with x-go-type-string-parse",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
				Extensions: map[string]interface{}{
					"x-go-type-string-parse": "time.Parse",
				},
			},
			expected: true,
		},
		{
			name: "Regular string",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
			expected: false,
		},
		{
			name: "Integer type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"integer"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typ.isCustomType(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestType_fillGoType_BasicTypes(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name       string
		schema     *openapi3.Schema
		expectFunc func(t *testing.T, stmt *jen.Statement)
	}{
		{
			name: "String type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
			expectFunc: func(t *testing.T, stmt *jen.Statement) {
				code := stmt.GoString()
				assert.Contains(t, code, "string")
			},
		},
		{
			name: "Integer type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"integer"},
			},
			expectFunc: func(t *testing.T, stmt *jen.Statement) {
				code := stmt.GoString()
				assert.Contains(t, code, "int")
			},
		},
		{
			name: "Number type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"number"},
			},
			expectFunc: func(t *testing.T, stmt *jen.Statement) {
				code := stmt.GoString()
				assert.Contains(t, code, "float64")
			},
		},
		{
			name: "Boolean type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"boolean"},
			},
			expectFunc: func(t *testing.T, stmt *jen.Statement) {
				code := stmt.GoString()
				assert.Contains(t, code, "bool")
			},
		},
		{
			name: "Array type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"array"},
				Items: &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"string"},
					},
				},
			},
			expectFunc: func(t *testing.T, stmt *jen.Statement) {
				code := stmt.GoString()
				assert.Contains(t, code, "[]")
				assert.Contains(t, code, "string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := &jen.Statement{}
			schemaRef := &openapi3.SchemaRef{Value: tt.schema}
			
			typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)
			tt.expectFunc(t, stmt)
		})
	}
}

func TestType_fillGoType_SpecialFormats(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name       string
		format     string
		expectType string
	}{
		{name: "UUID format", format: "uuid", expectType: "UUID"},
		{name: "Date format", format: "date", expectType: "string"},
		{name: "DateTime format", format: "date-time", expectType: "string"},
		{name: "Email format", format: "email", expectType: "string"},
		{name: "Byte format", format: "byte", expectType: "[]byte"},
		{name: "Binary format", format: "binary", expectType: "[]byte"},
		{name: "JSON format", format: "json", expectType: "RawMessage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &openapi3.Schema{
				Type:   &openapi3.Types{"string"},
				Format: tt.format,
			}
			schemaRef := &openapi3.SchemaRef{Value: schema}
			
			stmt := &jen.Statement{}
			typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)
			
			code := stmt.GoString()
			assert.Contains(t, code, tt.expectType)
		})
	}
}

func TestType_fillGoType_WithPointer(t *testing.T) {
	typ := setupTypeTest()

	schema := &openapi3.Schema{
		Type: &openapi3.Types{"string"},
		Extensions: map[string]interface{}{
			"x-go-pointer": true,
		},
	}
	schemaRef := &openapi3.SchemaRef{Value: schema}

	stmt := &jen.Statement{}
	typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)

	code := stmt.GoString()
	assert.Contains(t, code, "*string")
}

func TestType_fillGoType_WithCustomType(t *testing.T) {
	typ := setupTypeTest()

	schema := &openapi3.Schema{
		Extensions: map[string]interface{}{
			"x-go-type": "github.com/google/uuid.UUID",
		},
	}
	schemaRef := &openapi3.SchemaRef{Value: schema}

	stmt := &jen.Statement{}
	typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)

	code := stmt.GoString()
	assert.Contains(t, code, "UUID")
}

func TestType_hasXGoType(t *testing.T) {
	typ := setupTypeTest()

	tests := []struct {
		name     string
		schema   *openapi3.Schema
		expected bool
	}{
		{
			name: "Has x-go-type extension",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"x-go-type": "CustomType",
				},
			},
			expected: true,
		},
		{
			name: "No x-go-type extension",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
			expected: false,
		},
		{
			name: "Empty extensions",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typ.hasXGoType(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTagName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal camelCase",
			input:    "firstName",
			expected: "firstName",
		},
		{
			name:     "PascalCase",
			input:    "FirstName",
			expected: "firstName",
		},
		{
			name:     "All uppercase",
			input:    "USERNAME",
			expected: "username",
		},
		{
			name:     "Single character uppercase",
			input:    "A",
			expected: "a",
		},
		{
			name:     "Already lowercase",
			input:    "username",
			expected: "username",
		},
		{
			name:     "Mixed case",
			input:    "userNAME",
			expected: "userNAME",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTagName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test edge cases and error conditions
func TestType_EdgeCases(t *testing.T) {
	typ := setupTypeTest()

	t.Run("Nil schema", func(t *testing.T) {
		stmt := &jen.Statement{}
		schemaRef := &openapi3.SchemaRef{Value: nil}
		
		// Should not panic, might fill with interface{}
		require.NotPanics(t, func() {
			typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)
		})
	})

	t.Run("Schema with ref", func(t *testing.T) {
		stmt := &jen.Statement{}
		schemaRef := &openapi3.SchemaRef{
			Ref: "#/components/schemas/User",
			Value: &openapi3.Schema{}, // Add empty value so it's not nil
		}
		
		typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)
		code := stmt.GoString()
		assert.Contains(t, code, "User")
	})

	t.Run("Schema with oneOf", func(t *testing.T) {
		stmt := &jen.Statement{}
		schemaRef := &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				OneOf: []*openapi3.SchemaRef{
					{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
					{Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}}},
				},
			},
		}
		
		typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)
		code := stmt.GoString()
		assert.Contains(t, code, "interface{}")
	})
}

// Benchmark tests for performance-critical methods
func BenchmarkType_fillGoType(b *testing.B) {
	typ := setupTypeTest()
	schemaRef := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"string"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stmt := &jen.Statement{}
		typ.fillGoType(stmt, "", "TestType", schemaRef, false, false)
	}
}

func BenchmarkType_getXGoType(b *testing.B) {
	typ := setupTypeTest()
	schema := &openapi3.Schema{
		Extensions: map[string]interface{}{
			"x-go-type": "github.com/google/uuid.UUID",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		typ.getXGoType(schema)
	}
}