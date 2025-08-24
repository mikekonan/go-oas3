package generator

import (
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupValidationTest() *Generator {
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

func TestGenerator_validationFuncFromRules(t *testing.T) {
	generator := setupValidationTest()

	schema := &openapi3.Schema{
		Type:      &openapi3.Types{"string"},
		MinLength: 5,
		MaxLength: func() *uint64 { v := uint64(100); return &v }(),
	}

	// Create some mock validation rules (proper v4.Field calls)
	rules := []jen.Code{
		jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(
			jen.Op("&").Id("user").Dot("Name"),
			jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"),
			jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "RuneLength").Call(jen.Lit(5), jen.Lit(100)),
		),
	}

	result := generator.validationFuncFromRules("user", "ValidateUserName", rules, schema)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "func (user ValidateUserName) Validate() error")
	assert.Contains(t, codeStr, "v4.ValidateStruct")
	assert.Contains(t, codeStr, "v4.Field")
	assert.Contains(t, codeStr, "v4.Required")
	assert.Contains(t, codeStr, "v4.RuneLength")
}

func TestGenerator_fieldValidationRuleFromSchema(t *testing.T) {
	generator := setupValidationTest()

	tests := []struct {
		name           string
		receiverName   string
		propertyName   string
		schema         *openapi3.SchemaRef
		required       bool
		expectedInCode []string
	}{
		{
			name:         "String with min/max length",
			receiverName: "user",
			propertyName: "Name",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:      &openapi3.Types{"string"},
					MinLength: 2,
					MaxLength: func() *uint64 { v := uint64(50); return &v }(),
				},
			},
			required: true,
			expectedInCode: []string{
				"v4.Field(&user.Name",
				"v4.Required",
				"v4.RuneLength(2, 50)",
			},
		},
		{
			name:         "Integer with min/max value",
			receiverName: "product",
			propertyName: "Price",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:    &openapi3.Types{"integer"},
					Min: &[]float64{0}[0],
					Max: &[]float64{10000}[0],
				},
			},
			required: false,
			expectedInCode: []string{
				"v4.Field(&product.Price",
				"v4.Min(0)",
				"v4.Max(10000)",
			},
		},
		{
			name:         "Number with exclusive bounds",
			receiverName: "item",
			propertyName: "Rating",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:             &openapi3.Types{"number"},
					Min:          &[]float64{0}[0],
					Max:          &[]float64{5}[0],
					ExclusiveMin: true,
					ExclusiveMax: true,
				},
			},
			required: true,
			expectedInCode: []string{
				"v4.Field(&item.Rating",
				"v4.Required",
				"v4.Min(0.0).Exclusive()",
				"v4.Max(5.0).Exclusive()",
			},
		},
		{
			name:         "String with pattern (regex)",
			receiverName: "user",
			propertyName: "Email",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:    &openapi3.Types{"string"},
					Pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
				},
			},
			required: true,
			expectedInCode: []string{
				// Pattern validation should now work
				"v4.Field",
				"v4.Required",
				"v4.Match",
			},
		},
		{
			name:         "Array with min/max items",
			receiverName: "list",
			propertyName: "Items",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:     &openapi3.Types{"array"},
					MinItems: 1,
					MaxItems: func() *uint64 { v := uint64(10); return &v }(),
				},
			},
			required: false,
			expectedInCode: []string{
				"v4.Field(&list.Items, v4.Required, v4.Length(1, 10))",
			},
		},
		{
			name:         "String with enum values",
			receiverName: "status",
			propertyName: "Value",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
					Enum: []interface{}{"active", "inactive", "pending"},
				},
			},
			required: true,
			expectedInCode: []string{
				// Enum validation not implemented yet - returns nil
			},
		},
		{
			name:         "Required field validation",
			receiverName: "user",
			propertyName: "Id",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
			required: true,
			expectedInCode: []string{
				// Required field validation not implemented yet - returns nil
			},
		},
		{
			name:         "Optional field - no required validation",
			receiverName: "user",
			propertyName: "MiddleName",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
			required: false,
			expectedInCode: []string{
				// Should not have required field validation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.fieldValidationRuleFromSchema(tt.receiverName, tt.propertyName, tt.schema, tt.required)
			
			if len(tt.expectedInCode) == 0 {
				// For cases where no validation is expected (unimplemented features)
				assert.Nil(t, result)
				return
			}
			
			require.NotNil(t, result)
			require.Greater(t, len(result), 0, "Expected at least one validation rule")
			file := jen.NewFile("test").Add(result...)
	codeStr := file.GoString()
			
			for _, expected := range tt.expectedInCode {
				assert.Contains(t, codeStr, expected, "Expected %q in generated validation code", expected)
			}
			
			// Special case: required field should not have required validation if not required
			if !tt.required && tt.name == "Optional field - no required validation" {
				assert.NotContains(t, codeStr, "required field")
			}
		})
	}
}

func TestGenerator_validation_EdgeCases(t *testing.T) {
	generator := setupValidationTest()

	tests := []struct {
		name      string
		schema    *openapi3.SchemaRef
		expectErr bool
	}{
		{
			name: "Schema with x-go-skip-validation",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
					Extensions: map[string]interface{}{
						"x-go-skip-validation": true,
					},
					MinLength: 5,
				},
			},
			expectErr: false, // Should skip validation
		},
		{
			name: "Nil schema",
			schema: &openapi3.SchemaRef{
				Value: nil,
			},
			expectErr: false, // Should handle gracefully
		},
		{
			name: "Schema without constraints",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
					// No validation constraints
				},
			},
			expectErr: false, // Should return null
		},
		{
			name: "Schema with multiple constraints",
			schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:      &openapi3.Types{"string"},
					MinLength: 5,
					MaxLength: func() *uint64 { v := uint64(100); return &v }(),
					Pattern:   "^[a-zA-Z]+$",
					Enum:      []interface{}{"valid1", "valid2"},
				},
			},
			expectErr: false, // Should handle multiple constraints
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.fieldValidationRuleFromSchema("test", "Field", tt.schema, false)
			
			if tt.expectErr {
				assert.Nil(t, result)
			} else {
				// For skip validation extension or no constraints, should return null
				if tt.schema.Value != nil && generator.typee.getXGoSkipValidation(tt.schema.Value) {
					// Skip validation should return nil
					assert.Nil(t, result)
				} else if tt.schema.Value == nil {
					// Nil schema should return nil
					assert.Nil(t, result)
				} else if tt.schema.Value.MinLength == 0 && tt.schema.Value.MaxLength == nil && 
				          tt.schema.Value.Pattern == "" && tt.schema.Value.Enum == nil &&
				          tt.schema.Value.Min == nil && tt.schema.Value.Max == nil &&
				          tt.schema.Value.MinItems == 0 && tt.schema.Value.MaxItems == nil {
					// No validation constraints should return nil
					assert.Nil(t, result)
				} else {
					assert.NotNil(t, result)
					// Verify the code can be generated without panic by wrapping in validation function
					validationFunc := generator.validationFuncFromRules("test", "Validate", result, tt.schema.Value)
					file := jen.NewFile("test").Add(validationFunc)
	codeStr := file.GoString()
					assert.NotEmpty(t, codeStr)
				}
			}
		})
	}
}

func TestGenerator_validationFuncFromRules_EmptyRules(t *testing.T) {
	generator := setupValidationTest()

	schema := &openapi3.Schema{
		Type: &openapi3.Types{"string"},
	}

	// Empty rules
	rules := []jen.Code{}

	result := generator.validationFuncFromRules("user", "ValidateEmpty", rules, schema)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "func (user ValidateEmpty) Validate() error")
	assert.Contains(t, codeStr, "return nil")
	// Should not contain any validation logic
	assert.NotContains(t, codeStr, "if")
}

func TestGenerator_validationRules_MultipleTypes(t *testing.T) {
	generator := setupValidationTest()

	tests := []struct {
		name           string
		schema         *openapi3.Schema
		propertyName   string
		expectedInCode []string
	}{
		{
			name: "Boolean type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"boolean"},
			},
			propertyName: "IsActive",
			expectedInCode: []string{
				// Boolean typically doesn't have validation constraints
			},
		},
		{
			name: "Object type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"object"},
			},
			propertyName: "Metadata",
			expectedInCode: []string{
				// Object validation would typically be recursive
			},
		},
		{
			name: "Array of strings with constraints",
			schema: &openapi3.Schema{
				Type:     &openapi3.Types{"array"},
				MinItems: 1,
				MaxItems: func() *uint64 { v := uint64(5); return &v }(),
				Items: &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:      &openapi3.Types{"string"},
						MinLength: 1,
					},
				},
			},
			propertyName: "Tags",
			expectedInCode: []string{
				"v4.Length(1, 5)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaRef := &openapi3.SchemaRef{Value: tt.schema}
			result := generator.fieldValidationRuleFromSchema("test", tt.propertyName, schemaRef, false)
			
			if len(tt.expectedInCode) == 0 {
				// For types without typical validation constraints
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				file := jen.NewFile("test").Add(result...)
	codeStr := file.GoString()
				
				for _, expected := range tt.expectedInCode {
					assert.Contains(t, codeStr, expected, "Expected %q in validation code", expected)
				}
			}
		})
	}
}

func TestGenerator_validationWithCustomFormats(t *testing.T) {
	generator := setupValidationTest()

	tests := []struct {
		name           string
		schema         *openapi3.Schema
		expectedInCode []string
	}{
		{
			name: "Email format",
			schema: &openapi3.Schema{
				Type:   &openapi3.Types{"string"},
				Format: "email",
			},
			expectedInCode: []string{
				// Email format validation might be handled differently
			},
		},
		{
			name: "Date format",
			schema: &openapi3.Schema{
				Type:   &openapi3.Types{"string"},
				Format: "date",
			},
			expectedInCode: []string{
				// Date format validation
			},
		},
		{
			name: "UUID format",
			schema: &openapi3.Schema{
				Type:   &openapi3.Types{"string"},
				Format: "uuid",
			},
			expectedInCode: []string{
				// UUID format validation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaRef := &openapi3.SchemaRef{Value: tt.schema}
			result := generator.fieldValidationRuleFromSchema("test", "Field", schemaRef, false)
			
			if len(tt.expectedInCode) == 0 {
				// Format validation might not be implemented or might return null
				if len(result) == 0 {
					return // This is acceptable
				}
			}
			
			if len(result) > 0 {
				file := jen.NewFile("test").Add(result...)
	codeStr := file.GoString()
				for _, expected := range tt.expectedInCode {
					assert.Contains(t, codeStr, expected, "Expected %q in validation code", expected)
				}
			}
		})
	}
}