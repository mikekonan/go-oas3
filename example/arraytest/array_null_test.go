package arraytest

import (
	"encoding/json"
	"testing"
)

func TestArrayNullHandling(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		expectError bool
		expectation func(*testing.T, ArrayTestRequest)
	}{
		{
			name:        "Required array fields missing should error",
			jsonInput:   `{}`,
			expectError: true,
		},
		{
			name:        "Required string array null should error",
			jsonInput:   `{"requiredStringArray": null, "requiredIntArray": [1,2,3]}`,
			expectError: true,
		},
		{
			name:        "Required int array null should error",
			jsonInput:   `{"requiredStringArray": ["a","b"], "requiredIntArray": null}`,
			expectError: true,
		},
		{
			name:        "Both required arrays null should error",
			jsonInput:   `{"requiredStringArray": null, "requiredIntArray": null}`,
			expectError: true,
		},
		{
			name:        "Required arrays empty should pass",
			jsonInput:   `{"requiredStringArray": [], "requiredIntArray": []}`,
			expectError: false,
			expectation: func(t *testing.T, req ArrayTestRequest) {
				if len(req.RequiredStringArray) != 0 {
					t.Errorf("Expected empty RequiredStringArray, got %v", req.RequiredStringArray)
				}
				if len(req.RequiredIntArray) != 0 {
					t.Errorf("Expected empty RequiredIntArray, got %v", req.RequiredIntArray)
				}
				if req.OptionalStringArray != nil {
					t.Errorf("Expected nil OptionalStringArray, got %v", req.OptionalStringArray)
				}
			},
		},
		{
			name:        "Required arrays with values should pass",
			jsonInput:   `{"requiredStringArray": ["hello", "world"], "requiredIntArray": [1,2,3]}`,
			expectError: false,
			expectation: func(t *testing.T, req ArrayTestRequest) {
				if len(req.RequiredStringArray) != 2 {
					t.Errorf("Expected 2 items in RequiredStringArray, got %d", len(req.RequiredStringArray))
				}
				if len(req.RequiredIntArray) != 3 {
					t.Errorf("Expected 3 items in RequiredIntArray, got %d", len(req.RequiredIntArray))
				}
			},
		},
		{
			name:        "Optional array null should result in nil",
			jsonInput:   `{"requiredStringArray": ["a"], "requiredIntArray": [1], "optionalStringArray": null}`,
			expectError: false,
			expectation: func(t *testing.T, req ArrayTestRequest) {
				if req.OptionalStringArray != nil {
					t.Errorf("Expected nil OptionalStringArray when null, got %v", req.OptionalStringArray)
				}
			},
		},
		{
			name:        "Optional array missing should result in nil",
			jsonInput:   `{"requiredStringArray": ["a"], "requiredIntArray": [1]}`,
			expectError: false,
			expectation: func(t *testing.T, req ArrayTestRequest) {
				if req.OptionalStringArray != nil {
					t.Errorf("Expected nil OptionalStringArray when missing, got %v", req.OptionalStringArray)
				}
			},
		},
		{
			name:        "Optional array empty should result in empty slice",
			jsonInput:   `{"requiredStringArray": ["a"], "requiredIntArray": [1], "optionalStringArray": []}`,
			expectError: false,
			expectation: func(t *testing.T, req ArrayTestRequest) {
				if req.OptionalStringArray == nil {
					t.Errorf("Expected empty slice OptionalStringArray, got nil")
				} else if len(req.OptionalStringArray) != 0 {
					t.Errorf("Expected empty OptionalStringArray, got %v", req.OptionalStringArray)
				}
			},
		},
		{
			name:        "Optional array with values should work",
			jsonInput:   `{"requiredStringArray": ["a"], "requiredIntArray": [1], "optionalStringArray": ["x", "y"]}`,
			expectError: false,
			expectation: func(t *testing.T, req ArrayTestRequest) {
				if len(req.OptionalStringArray) != 2 {
					t.Errorf("Expected 2 items in OptionalStringArray, got %d", len(req.OptionalStringArray))
				}
			},
		},
		{
			name:        "Complex case with object array",
			jsonInput:   `{"requiredStringArray": ["test"], "requiredIntArray": [42], "objectArray": [{"id": "123", "name": "test"}]}`,
			expectError: false,
			expectation: func(t *testing.T, req ArrayTestRequest) {
				if len(req.ObjectArray) != 1 {
					t.Errorf("Expected 1 item in ObjectArray, got %d", len(req.ObjectArray))
				}
				if len(req.ObjectArray) > 0 && req.ObjectArray[0].ID != "123" {
					t.Errorf("Expected object ID '123', got '%s'", req.ObjectArray[0].ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ArrayTestRequest
			err := json.Unmarshal([]byte(tt.jsonInput), &req)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none for input: %s", tt.jsonInput)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v for input: %s", err, tt.jsonInput)
				} else if tt.expectation != nil {
					tt.expectation(t, req)
				}
			}
		})
	}
}

func TestArrayNullVsEmptyVsMissing(t *testing.T) {
	// Test the three distinct states for arrays
	
	// Test 1: null array
	jsonNull := `{"requiredStringArray": ["a"], "requiredIntArray": [1], "optionalStringArray": null}`
	var reqNull ArrayTestRequest
	err := json.Unmarshal([]byte(jsonNull), &reqNull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if reqNull.OptionalStringArray != nil {
		t.Errorf("Expected null array to be nil, got %v", reqNull.OptionalStringArray)
	}
	
	// Test 2: missing array
	jsonMissing := `{"requiredStringArray": ["a"], "requiredIntArray": [1]}`
	var reqMissing ArrayTestRequest
	err = json.Unmarshal([]byte(jsonMissing), &reqMissing)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if reqMissing.OptionalStringArray != nil {
		t.Errorf("Expected missing array to be nil, got %v", reqMissing.OptionalStringArray)
	}
	
	// Test 3: empty array
	jsonEmpty := `{"requiredStringArray": ["a"], "requiredIntArray": [1], "optionalStringArray": []}`
	var reqEmpty ArrayTestRequest
	err = json.Unmarshal([]byte(jsonEmpty), &reqEmpty)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if reqEmpty.OptionalStringArray == nil {
		t.Errorf("Expected empty array to be empty slice, got nil")
	} else if len(reqEmpty.OptionalStringArray) != 0 {
		t.Errorf("Expected empty array to have 0 length, got %d", len(reqEmpty.OptionalStringArray))
	}
	
	t.Logf("✓ null array:    %v (nil=%t)", reqNull.OptionalStringArray, reqNull.OptionalStringArray == nil)
	t.Logf("✓ missing array: %v (nil=%t)", reqMissing.OptionalStringArray, reqMissing.OptionalStringArray == nil)
	t.Logf("✓ empty array:   %v (nil=%t, len=%d)", reqEmpty.OptionalStringArray, reqEmpty.OptionalStringArray == nil, len(reqEmpty.OptionalStringArray))
}