package example

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNullHandling(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectError bool
		errorMsg    string
		expectValid bool
		description string
	}{
		{
			name:        "required field null",
			json:        `{"description": null, "title": "test title", "regexParam": "123"}`,
			expectError: true,
			errorMsg:    "field 'Description' is required but was null or missing",
			description: "Required field set to null should produce descriptive error",
		},
		{
			name:        "required field missing",
			json:        `{"title": "test title", "regexParam": "123"}`,
			expectError: true,
			errorMsg:    "field 'Description' is required but was null or missing",
			description: "Missing required field should produce descriptive error",
		},
		{
			name:        "optional field null",
			json:        `{"description": "test description", "regexParam": "123"}`,
			expectError: false,
			expectValid: true,
			description: "Optional field null should be handled gracefully",
		},
		{
			name:        "optional field missing",
			json:        `{"description": "test description", "regexParam": "123"}`,
			expectError: false,
			expectValid: true,
			description: "Missing optional field should be handled gracefully",
		},
		{
			name:        "required field with valid value",
			json:        `{"description": "test description", "regexParam": "123"}`,
			expectError: false,
			expectValid: true,
			description: "Required field with valid value should work",
		},
		{
			name:        "required field with empty string (should fail validation)",
			json:        `{"description": "short", "regexParam": "123"}`,
			expectError: false, // UnmarshalJSON should succeed
			expectValid: false, // but Validate() should fail due to minLength: 8
			description: "Required field with empty/short string should pass UnmarshalJSON but fail validation",
		},
		{
			name:        "optional pointer field null",
			json:        `{"description": "test description", "details": null, "regexParam": "123"}`,
			expectError: false,
			expectValid: true,
			description: "Optional pointer field set to null should be handled gracefully",
		},
		{
			name:        "optional pointer field missing",
			json:        `{"description": "test description", "regexParam": "123"}`,
			expectError: false,
			expectValid: true,
			description: "Missing optional pointer field should be handled gracefully",
		},
		{
			name:        "optional pointer field with value",
			json:        `{"description": "test description", "details": "some details", "regexParam": "123"}`,
			expectError: false,
			expectValid: true,
			description: "Optional pointer field with value should work correctly",
		},
		{
			name:        "regex field validation failure",
			json:        `{"description": "test description", "regexParam": "invalid"}`,
			expectError: true,
			errorMsg:    "field 'RegexParam' does not match pattern",
			description: "Field not matching regex should produce descriptive error",
		},
		{
			name:        "regex field validation success",
			json:        `{"description": "test description", "regexParam": "123"}`,
			expectError: false,
			expectValid: true,
			description: "Field matching regex should work correctly",
		},
		{
			name:        "malformed json",
			json:        `{"description": "test", invalid}`,
			expectError: true,
			description: "Malformed JSON should produce parsing error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req CreateTransactionRequest
			err := json.Unmarshal([]byte(tt.json), &req)

			// Test UnmarshalJSON behavior
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none for %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for %s: %v", tt.description, err)
			}
			if tt.expectError && err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error message containing %q but got %q for %s", tt.errorMsg, err.Error(), tt.description)
			}

			// Test validation behavior for cases that pass UnmarshalJSON
			if !tt.expectError && err == nil {
				validationErr := req.Validate()
				if tt.expectValid && validationErr != nil {
					t.Errorf("expected validation to pass but got error for %s: %v", tt.description, validationErr)
				}
				if !tt.expectValid && validationErr == nil {
					t.Errorf("expected validation to fail but got no error for %s", tt.description)
				}
			}
		})
	}
}

func TestUpdateTransactionNullHandling(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectError bool
		errorMsg    string
		description string
	}{
		{
			name:        "required description field null",
			json:        `{"description": null, "title": "test title"}`,
			expectError: true,
			errorMsg:    "field 'Description' is required but was null or missing",
			description: "Required field set to null should produce descriptive error",
		},
		{
			name:        "required description field missing",
			json:        `{"title": "test title"}`,
			expectError: true,
			errorMsg:    "field 'Description' is required but was null or missing",
			description: "Missing required field should produce descriptive error",
		},
		{
			name:        "valid update request",
			json:        `{"description": "updated description", "title": "updated title"}`,
			expectError: false,
			description: "Valid update request should work",
		},
		{
			name:        "optional details field null",
			json:        `{"description": "updated description", "details": null}`,
			expectError: false,
			description: "Optional pointer field null should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req UpdateTransactionRequest
			err := json.Unmarshal([]byte(tt.json), &req)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none for %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for %s: %v", tt.description, err)
			}
			if tt.expectError && err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error message containing %q but got %q for %s", tt.errorMsg, err.Error(), tt.description)
			}
		})
	}
}

func TestNullSemanticsCorrectness(t *testing.T) {
	// Test the three-state problem: missing vs null vs empty string
	t.Run("distinguish missing, null, and empty", func(t *testing.T) {
		testCases := []struct {
			name     string
			json     string
			expected string
		}{
			{
				name:     "field missing",
				json:     `{"description": "test desc", "regexParam": "123"}`,
				expected: "missing",
			},
			{
				name:     "field null", 
				json:     `{"description": "test desc", "details": null, "regexParam": "123"}`,
				expected: "null",
			},
			{
				name:     "field empty string",
				json:     `{"description": "test desc", "details": "", "regexParam": "123"}`,
				expected: "empty",
			},
			{
				name:     "field with value",
				json:     `{"description": "test desc", "details": "some value", "regexParam": "123"}`,
				expected: "value",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var req CreateTransactionRequest
				err := json.Unmarshal([]byte(tc.json), &req)
				if err != nil {
					t.Fatalf("unexpected unmarshal error: %v", err)
				}

				switch tc.expected {
				case "missing", "null":
					if req.Details != nil {
						t.Errorf("expected Details to be nil for %s case, got: %v", tc.expected, req.Details)
					}
				case "empty":
					if req.Details == nil {
						t.Errorf("expected Details to be non-nil for empty string case")
					} else if *req.Details != "" {
						t.Errorf("expected Details to be empty string, got: %q", *req.Details)
					}
				case "value":
					if req.Details == nil {
						t.Errorf("expected Details to be non-nil for value case")
					} else if *req.Details != "some value" {
						t.Errorf("expected Details to be 'some value', got: %q", *req.Details)
					}
				}
			})
		}
	})
}

func BenchmarkUnmarshalJSON(b *testing.B) {
	data := []byte(`{"description":"test description","title":"test title","amount":100.0,"amountCents":50,"email":"test@example.com","transactionID":"550e8400-e29b-41d4-a716-446655440000","regexParam":"123"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var req CreateTransactionRequest
		if err := json.Unmarshal(data, &req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidation(b *testing.B) {
	req := CreateTransactionRequest{
		Description:   "test description",
		Title:         "test title",
		Amount:        100.0,
		AmountCents:   50,
		RegexParam:    "123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := req.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}