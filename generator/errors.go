package generator

import (
	"fmt"
	"runtime"

	"github.com/cockroachdb/errors"
)

// PanicWithContext creates a detailed panic with stack trace and context information
func PanicWithContext(operation string, details map[string]interface{}, baseErr error) {
	// Get caller information
	_, file, line, ok := runtime.Caller(1)
	callerInfo := "unknown"
	if ok {
		callerInfo = fmt.Sprintf("%s:%d", file, line)
	}
	
	// Create base error if none provided
	if baseErr == nil {
		baseErr = errors.Newf("generator panic in %s", operation)
	}
	
	// Add detailed context
	contextualErr := errors.WithDetailf(baseErr, "🚨 Generator Error Details")
	contextualErr = errors.WithDetailf(contextualErr, "📍 Location: %s", callerInfo)
	contextualErr = errors.WithDetailf(contextualErr, "🔧 Operation: %s", operation)
	
	// Add all context details
	if len(details) > 0 {
		contextualErr = errors.WithDetailf(contextualErr, "📋 Context:")
		for key, value := range details {
			contextualErr = errors.WithDetailf(contextualErr, "  • %s: %v", key, value)
		}
	}
	
	// Add hints for common issues
	contextualErr = errors.WithHintf(contextualErr, "💡 This usually indicates a schema validation issue or unexpected OpenAPI extension format")
	contextualErr = errors.WithHintf(contextualErr, "🔍 Check the OpenAPI specification around the mentioned field/extension")
	
	// Create detailed panic message with beautiful formatting
	panicMsg := fmt.Sprintf("\n%s", errors.FlattenDetails(contextualErr))
	panic(panicMsg)
}

// PanicUnexpectedExtensionType creates a panic for unexpected extension types with detailed context
func PanicUnexpectedExtensionType(extensionName string, receivedType interface{}, schemaContext map[string]interface{}) {
	details := map[string]interface{}{
		ContextExtensionName: extensionName,
		ContextReceivedType:  fmt.Sprintf("%T", receivedType),
		ContextReceivedValue: receivedType,
		ContextExpectedTypes: "string, json.RawMessage, or bool",
	}
	
	// Add schema context if provided
	for key, value := range schemaContext {
		details["schema_"+key] = value
	}
	
	baseErr := errors.Newf("unexpected type %T for extension %s", receivedType, extensionName)
	
	PanicWithContext("Extension Type Validation", details, baseErr)
}

// PanicInvalidOperation creates a panic for invalid operations with operation context
func PanicInvalidOperation(operation string, reason string, context map[string]interface{}) {
	details := map[string]interface{}{
		ContextOperation: operation,
		ContextReason:    reason,
	}
	
	// Add additional context
	for key, value := range context {
		details[key] = value
	}
	
	baseErr := errors.Newf("invalid operation: %s", operation)
	baseErr = errors.WithDetailf(baseErr, "Reason: %s", reason)
	baseErr = errors.WithHintf(baseErr, "🔧 Review the operation logic and input parameters")
	
	PanicWithContext("Operation Validation", details, baseErr)
}

// PanicSchemaValidation creates a panic for schema validation errors
func PanicSchemaValidation(schemaType string, fieldName string, issue string, schemaDetails map[string]interface{}) {
	details := map[string]interface{}{
		ContextSchemaType: schemaType,
		ContextFieldName:  fieldName,
		"issue":           issue,
	}
	
	// Add schema details
	for key, value := range schemaDetails {
		details["schema_"+key] = value
	}
	
	baseErr := errors.Newf("schema validation failed for %s.%s", schemaType, fieldName)
	baseErr = errors.WithDetailf(baseErr, "Issue: %s", issue)
	baseErr = errors.WithHintf(baseErr, "🔧 Check the OpenAPI schema definition for field '%s'", fieldName)
	baseErr = errors.WithHintf(baseErr, "📋 Review schema type '%s' for correctness", schemaType)
	
	PanicWithContext("Schema Validation", details, baseErr)
}

// PanicOperationError creates a panic for operation errors (like JSON unmarshaling, merging, etc.)
func PanicOperationError(operation string, err error, operationContext map[string]interface{}) {
	details := map[string]interface{}{
		ContextOperation: operation,
		"error_message": err.Error(),
	}
	
	// Add operation context
	for key, value := range operationContext {
		details[key] = value
	}
	
	baseErr := errors.Wrapf(err, "operation failed: %s", operation)
	baseErr = errors.WithHintf(baseErr, "🔧 This is likely a data parsing or schema merging issue")
	baseErr = errors.WithHintf(baseErr, "📋 Check the input data format and schema structure")
	
	PanicWithContext("Operation Error", details, baseErr)
}