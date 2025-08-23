package generator

import (
	"fmt"
	"runtime"

	"github.com/cockroachdb/errors"
)

// PanicWithContext panics with a formatted, flattened error that includes the operation name, caller location, supplied contextual key/value pairs, and helpful hints.
// If baseErr is nil a default error "generator panic in <operation>" is used. Caller location is formatted as "file:line" when available or ValueUnknown when not.
// The entries in details are rendered as individual lines in the panic message to aid debugging.
func PanicWithContext(operation string, details map[string]interface{}, baseErr error) {
	// Get caller information
	_, file, line, ok := runtime.Caller(1)
	callerInfo := ValueUnknown
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

// PanicUnexpectedExtensionType panics with a richly detailed error when an OpenAPI extension value has an unexpected Go type.
// The produced error includes the extension name, the received value and its Go type, the allowed types ("string, json.RawMessage, or bool"),
// and any provided schemaContext entries (each added to the details map with the ContextSchemaPrefix).
func PanicUnexpectedExtensionType(extensionName string, receivedType interface{}, schemaContext map[string]interface{}) {
	details := map[string]interface{}{
		ContextExtensionName: extensionName,
		ContextReceivedType:  fmt.Sprintf("%T", receivedType),
		ContextReceivedValue: receivedType,
		ContextExpectedTypes: "string, json.RawMessage, or bool",
	}
	
	// Add schema context if provided
	for key, value := range schemaContext {
		details[ContextSchemaPrefix+key] = value
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

// PanicSchemaValidation records schema-related context and panics with a detailed error.
//
// PanicSchemaValidation builds a details map containing ContextSchemaType, ContextFieldName,
// and ContextIssue, merges each entry from schemaDetails using keys prefixed with
// ContextSchemaPrefix (i.e. ContextSchemaPrefix+key), and constructs a base error
// "schema validation failed for <schemaType>.<fieldName>" with the issue as a detail and
// two hints about checking the OpenAPI schema and schema type. It then delegates to
// PanicWithContext with operation "Schema Validation" and panics with the flattened message.
//
// Parameters are self-descriptive: schemaType is the schema/object name, fieldName is the
// field with the validation problem, issue is a short description of the problem, and
// schemaDetails contains additional schema-specific context that will be namespaced with
// ContextSchemaPrefix in the emitted error.
func PanicSchemaValidation(schemaType string, fieldName string, issue string, schemaDetails map[string]interface{}) {
	details := map[string]interface{}{
		ContextSchemaType: schemaType,
		ContextFieldName:  fieldName,
		ContextIssue: issue,
	}
	
	// Add schema details
	for key, value := range schemaDetails {
		details[ContextSchemaPrefix+key] = value
	}
	
	baseErr := errors.Newf("schema validation failed for %s.%s", schemaType, fieldName)
	baseErr = errors.WithDetailf(baseErr, "Issue: %s", issue)
	baseErr = errors.WithHintf(baseErr, "🔧 Check the OpenAPI schema definition for field '%s'", fieldName)
	baseErr = errors.WithHintf(baseErr, "📋 Review schema type '%s' for correctness", schemaType)
	
	PanicWithContext("Schema Validation", details, baseErr)
}

// PanicOperationError wraps an operation-level error with contextual details and panics.
//
// PanicOperationError should be called when an operation (for example JSON unmarshaling,
// merging, or other data-processing steps) fails and you want to produce a rich, contextual
// panic message for debugging.
//
// Parameters:
//   - operation: a short identifier for the failing operation.
//   - err: the original error (must be non-nil); its message is recorded and the error is wrapped.
//   - operationContext: additional key/value pairs that will be merged into the panic's context.
//
// The resulting panic contains a wrapped error with hints and a details map that includes the
// operation name, the original error message, and any provided operationContext entries.
func PanicOperationError(operation string, err error, operationContext map[string]interface{}) {
	details := map[string]interface{}{
		ContextOperation: operation,
		ContextErrorMessage: err.Error(),
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