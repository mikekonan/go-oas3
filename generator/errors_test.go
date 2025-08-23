package generator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanicWithContext(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		details   map[string]interface{}
		baseErr   error
		shouldContain []string
	}{
		{
			name:      "Basic panic with context",
			operation: "TestOperation",
			details: map[string]interface{}{
				"field": "test_field",
				"value": 123,
			},
			baseErr: errors.New("test error"),
			shouldContain: []string{
				"TestOperation",
				"test_field",
				"123",
			},
		},
		{
			name:      "Panic without base error",
			operation: "TestOperation",
			details: map[string]interface{}{
				"context": "test_context",
			},
			baseErr: nil,
			shouldContain: []string{
				"TestOperation",
				"test_context",
			},
		},
		{
			name:      "Panic with empty details",
			operation: "EmptyDetails",
			details:   map[string]interface{}{},
			baseErr:   errors.New("base error"),
			shouldContain: []string{
				"EmptyDetails",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					panicMsg := r.(string)
					for _, shouldContain := range tt.shouldContain {
						assert.Contains(t, panicMsg, shouldContain)
					}
					// Check for common formatting elements
					assert.Contains(t, panicMsg, "🚨")
					assert.Contains(t, panicMsg, "📍")
					assert.Contains(t, panicMsg, "🔧")
				} else {
					t.Error("Expected panic but none occurred")
				}
			}()

			PanicWithContext(tt.operation, tt.details, tt.baseErr)
		})
	}
}

func TestPanicUnexpectedExtensionType(t *testing.T) {
	tests := []struct {
		name          string
		extensionName string
		receivedType  interface{}
		schemaContext map[string]interface{}
		shouldContain []string
	}{
		{
			name:          "Integer received instead of string",
			extensionName: "x-go-type",
			receivedType:  123,
			schemaContext: map[string]interface{}{
				"schema_type": "User",
				"field_name":  "id",
			},
			shouldContain: []string{
				"x-go-type",
				"int",
				"123",
				"schema_schema_type",
				"User",
			},
		},
		{
			name:          "Float received",
			extensionName: "x-go-pointer",
			receivedType:  3.14,
			schemaContext: map[string]interface{}{
				"field": "test_field",
			},
			shouldContain: []string{
				"x-go-pointer",
				"float64",
				"3.14",
				"schema_field",
				"test_field",
			},
		},
		{
			name:          "Empty schema context",
			extensionName: "x-go-omitempty",
			receivedType:  []string{"array"},
			schemaContext: map[string]interface{}{},
			shouldContain: []string{
				"x-go-omitempty",
				"[]string",
				"string, json.RawMessage, or bool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					panicMsg := r.(string)
					for _, shouldContain := range tt.shouldContain {
						assert.Contains(t, panicMsg, shouldContain)
					}
					// Check for standard formatting
					assert.Contains(t, panicMsg, "🔧")
				} else {
					t.Error("Expected panic but none occurred")
				}
			}()

			PanicUnexpectedExtensionType(tt.extensionName, tt.receivedType, tt.schemaContext)
		})
	}
}

func TestPanicInvalidOperation(t *testing.T) {
	tests := []struct {
		name          string
		operation     string
		reason        string
		context       map[string]interface{}
		shouldContain []string
	}{
		{
			name:      "Invalid schema merge",
			operation: "Schema Merge",
			reason:    "conflicting types",
			context: map[string]interface{}{
				"schema_a": "User",
				"schema_b": "Product",
			},
			shouldContain: []string{
				"Schema Merge",
				"conflicting types",
				"schema_a",
				"User",
			},
		},
		{
			name:      "Missing required field",
			operation: "Field Validation",
			reason:    "required field missing",
			context: map[string]interface{}{
				"field_name": "email",
				"schema":     "User",
			},
			shouldContain: []string{
				"Field Validation",
				"required field missing",
				"field_name",
				"email",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					panicMsg := r.(string)
					for _, shouldContain := range tt.shouldContain {
						assert.Contains(t, panicMsg, shouldContain)
					}
					// Check for operation-specific formatting
					assert.Contains(t, panicMsg, "🔧")
				} else {
					t.Error("Expected panic but none occurred")
				}
			}()

			PanicInvalidOperation(tt.operation, tt.reason, tt.context)
		})
	}
}

func TestPanicSchemaValidation(t *testing.T) {
	tests := []struct {
		name          string
		schemaType    string
		fieldName     string
		issue         string
		schemaDetails map[string]interface{}
		shouldContain []string
	}{
		{
			name:       "Invalid field type",
			schemaType: "User",
			fieldName:  "age",
			issue:      "type mismatch: expected integer, got string",
			schemaDetails: map[string]interface{}{
				"expected_type": "integer",
				"received_type": "string",
			},
			shouldContain: []string{
				"type mismatch",
				"schema_expected_type",
				"integer",
			},
		},
		{
			name:       "Missing required field",
			schemaType: "Product",
			fieldName:  "name",
			issue:      "required field is missing",
			schemaDetails: map[string]interface{}{
				"validation_rule": "required",
			},
			shouldContain: []string{
				"required field is missing",
				"schema_validation_rule",
				"required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					panicMsg := r.(string)
					for _, shouldContain := range tt.shouldContain {
						assert.Contains(t, panicMsg, shouldContain)
					}
					// Check for schema validation specific formatting
					assert.Contains(t, panicMsg, "🔧")
				} else {
					t.Error("Expected panic but none occurred")
				}
			}()

			PanicSchemaValidation(tt.schemaType, tt.fieldName, tt.issue, tt.schemaDetails)
		})
	}
}

func TestPanicOperationError(t *testing.T) {
	tests := []struct {
		name             string
		operation        string
		err              error
		operationContext map[string]interface{}
		shouldContain    []string
	}{
		{
			name:      "JSON unmarshal error",
			operation: "JSON Unmarshal",
			err:       errors.New("invalid character 'x' looking for beginning of value"),
			operationContext: map[string]interface{}{
				"input_data": "{invalid_json}",
				"field":      "extensions",
			},
			shouldContain: []string{
				"JSON Unmarshal",
				"invalid character",
				"input_data",
				"{invalid_json}",
			},
		},
		{
			name:      "Schema merge error",
			operation: "Schema Merge",
			err:       errors.New("cannot merge incompatible schemas"),
			operationContext: map[string]interface{}{
				"schema_type_a": "object",
				"schema_type_b": "array",
			},
			shouldContain: []string{
				"Schema Merge",
				"cannot merge incompatible schemas",
				"schema_type_a",
				"object",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					panicMsg := r.(string)
					for _, shouldContain := range tt.shouldContain {
						assert.Contains(t, panicMsg, shouldContain)
					}
					// Check for operation error specific formatting
					assert.Contains(t, panicMsg, "🔧")
				} else {
					t.Error("Expected panic but none occurred")
				}
			}()

			PanicOperationError(tt.operation, tt.err, tt.operationContext)
		})
	}
}

// Test that all panic functions include stack trace information
func TestPanicFunctions_IncludeStackTrace(t *testing.T) {
	testCases := []struct {
		name     string
		panicFunc func()
	}{
		{
			name: "PanicWithContext",
			panicFunc: func() {
				PanicWithContext("Test", map[string]interface{}{"key": "value"}, nil)
			},
		},
		{
			name: "PanicUnexpectedExtensionType",
			panicFunc: func() {
				PanicUnexpectedExtensionType("x-go-type", 123, map[string]interface{}{})
			},
		},
		{
			name: "PanicInvalidOperation",
			panicFunc: func() {
				PanicInvalidOperation("TestOp", "test reason", map[string]interface{}{})
			},
		},
		{
			name: "PanicSchemaValidation",
			panicFunc: func() {
				PanicSchemaValidation("User", "field", "test issue", map[string]interface{}{})
			},
		},
		{
			name: "PanicOperationError",
			panicFunc: func() {
				PanicOperationError("TestOp", errors.New("test error"), map[string]interface{}{})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					panicMsg := r.(string)
					// Check that stack trace information is included
					assert.Contains(t, panicMsg, "📍 Location:")
					assert.Contains(t, panicMsg, ".go:")
					// Note: hints are added by PanicWithContext
				} else {
					t.Error("Expected panic but none occurred")
				}
			}()

			tc.panicFunc()
		})
	}
}

// Test edge cases for error functions
func TestErrorFunctions_EdgeCases(t *testing.T) {
	t.Run("PanicWithContext with nil details", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				panicMsg := r.(string)
				assert.Contains(t, panicMsg, "TestOperation")
				// Should handle nil details gracefully
			} else {
				t.Error("Expected panic but none occurred")
			}
		}()

		PanicWithContext("TestOperation", nil, errors.New("test"))
	})

	t.Run("PanicUnexpectedExtensionType with nil type", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				panicMsg := r.(string)
				assert.Contains(t, panicMsg, "x-test-ext")
				assert.Contains(t, panicMsg, "<nil>")
			} else {
				t.Error("Expected panic but none occurred")
			}
		}()

		PanicUnexpectedExtensionType("x-test-ext", nil, map[string]interface{}{})
	})

	t.Run("PanicOperationError with nil error", func(t *testing.T) {
		// This will panic because nil error is passed, which is expected
		require.Panics(t, func() {
			PanicOperationError("TestOp", nil, map[string]interface{}{})
		})
	})
}

// Benchmark tests for error functions (should be fast even during panic)
func BenchmarkPanicWithContext(b *testing.B) {
	details := map[string]interface{}{
		"field":  "test_field",
		"value":  123,
		"nested": map[string]string{"key": "value"},
	}
	baseErr := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		func() {
			defer func() {
				recover() // Ignore the panic for benchmarking
			}()
			PanicWithContext("BenchmarkOperation", details, baseErr)
		}()
	}
}

func BenchmarkPanicUnexpectedExtensionType(b *testing.B) {
	schemaContext := map[string]interface{}{
		"schema_type": "User",
		"field_name":  "id",
		"path":        "/users/{id}",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		func() {
			defer func() {
				recover() // Ignore the panic for benchmarking
			}()
			PanicUnexpectedExtensionType("x-go-type", 123, schemaContext)
		}()
	}
}