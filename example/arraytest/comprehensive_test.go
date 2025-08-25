package arraytest

import (
	"encoding/json"
	"testing"
)

func TestComprehensiveArrayHandling(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		expectUnmarshalError bool
		expectValidationError bool
		description string
	}{
		{
			name:        "Valid complete request",
			jsonInput:   `{"requiredStringArray": ["hello", "world"], "requiredIntArray": [10, 20, 30]}`,
			expectUnmarshalError: false,
			expectValidationError: false,
			description: "All fields valid should pass both unmarshal and validation",
		},
		{
			name:        "Required array null should fail unmarshal",
			jsonInput:   `{"requiredStringArray": null, "requiredIntArray": [10, 20]}`,
			expectUnmarshalError: true,
			expectValidationError: false,
			description: "Null handling happens in UnmarshalJSON, not in validation",
		},
		{
			name:        "Required array missing should fail unmarshal",
			jsonInput:   `{"requiredIntArray": [10, 20]}`,
			expectUnmarshalError: true,
			expectValidationError: false,
			description: "Missing required field handled in UnmarshalJSON",
		},
		{
			name:        "Empty required array should pass unmarshal but fail validation",
			jsonInput:   `{"requiredStringArray": [], "requiredIntArray": [10, 20]}`,
			expectUnmarshalError: false,
			expectValidationError: true,
			description: "Empty array for minItems > 0 should be caught by validation",
		},
		{
			name:        "Too few items should pass unmarshal but fail validation",
			jsonInput:   `{"requiredStringArray": ["test"], "requiredIntArray": [10]}`,
			expectUnmarshalError: false,
			expectValidationError: true,
			description: "RequiredIntArray needs minItems: 2",
		},
		{
			name:        "Too many items should pass unmarshal but fail validation",
			jsonInput:   `{"requiredStringArray": ["a","b","c","d","e","f","g","h","i","j","k"], "requiredIntArray": [10, 20]}`,
			expectUnmarshalError: false,
			expectValidationError: true,
			description: "RequiredStringArray has maxItems: 10",
		},
		{
			name:        "Optional arrays should work correctly",
			jsonInput:   `{"requiredStringArray": ["test"], "requiredIntArray": [10, 20], "optionalStringArray": ["a", "b"], "emptyAllowedArray": []}`,
			expectUnmarshalError: false,
			expectValidationError: false,
			description: "Optional arrays with valid values should pass",
		},
		{
			name:        "Optional array null should work",
			jsonInput:   `{"requiredStringArray": ["test"], "requiredIntArray": [10, 20], "optionalStringArray": null}`,
			expectUnmarshalError: false,
			expectValidationError: false,
			description: "Null optional arrays should be fine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ArrayTestRequest
			
			// Test unmarshaling
			unmarshalErr := json.Unmarshal([]byte(tt.jsonInput), &req)
			if tt.expectUnmarshalError {
				if unmarshalErr == nil {
					t.Errorf("Expected unmarshal error but got none. %s", tt.description)
				} else {
					t.Logf("✅ Expected unmarshal error: %v", unmarshalErr)
					return // Don't test validation if unmarshal failed
				}
			} else {
				if unmarshalErr != nil {
					t.Errorf("Expected no unmarshal error but got: %v. %s", unmarshalErr, tt.description)
					return
				}
			}
			
			// Test validation
			validationErr := req.Validate()
			if tt.expectValidationError {
				if validationErr == nil {
					t.Errorf("Expected validation error but got none. %s", tt.description)
				} else {
					t.Logf("✅ Expected validation error: %v", validationErr)
				}
			} else {
				if validationErr != nil {
					t.Errorf("Expected no validation error but got: %v. %s", validationErr, tt.description)
				} else {
					t.Logf("✅ No errors as expected")
				}
			}
		})
	}
}