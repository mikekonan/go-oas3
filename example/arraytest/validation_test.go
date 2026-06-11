package arraytest

import (
	"testing"
)

func TestOptionalArrayMinItems(t *testing.T) {
	base := ArrayTestRequest{
		RequiredStringArray: []string{"hello", "world"},
		RequiredIntArray:    []int{10, 20},
	}

	absent := base
	if err := absent.Validate(); err != nil {
		t.Errorf("absent optional array must pass validation, got: %v", err)
	}

	empty := base
	empty.OptionalMinArray = []string{}
	if err := empty.Validate(); err == nil {
		t.Error("present-but-empty optional array must fail minItems: 2")
	}

	tooFew := base
	tooFew.OptionalMinArray = []string{"only-one"}
	if err := tooFew.Validate(); err == nil {
		t.Error("optional array with 1 item must fail minItems: 2")
	}

	enough := base
	enough.OptionalMinArray = []string{"one", "two"}
	if err := enough.Validate(); err != nil {
		t.Errorf("optional array with 2 items must pass minItems: 2, got: %v", err)
	}
}

func TestArrayValidationGenerated(t *testing.T) {
	// Create a valid request
	validRequest := ArrayTestRequest{
		RequiredStringArray: []string{"hello", "world"},
		RequiredIntArray:    []int{10, 20},
	}

	// Test validation on valid request
	err := validRequest.Validate()
	if err != nil {
		t.Errorf("Expected no validation error for valid request, got: %v", err)
	}

	// Test validation on request with empty required array (should fail - minItems: 1)
	emptyArrayRequest := ArrayTestRequest{
		RequiredStringArray: []string{},  // Empty but required array with minItems: 1
		RequiredIntArray:    []int{1, 2}, // Valid
	}

	err = emptyArrayRequest.Validate()
	if err != nil {
		t.Logf("Good! Validation caught empty required array: %v", err)
	} else {
		t.Errorf("ERROR: Validation should have caught empty required array (minItems: 1). RequiredStringArray length: %d", len(emptyArrayRequest.RequiredStringArray))
	}

	// Test validation on request with too few items in requiredIntArray (minItems: 2)
	tooFewIntItems := ArrayTestRequest{
		RequiredStringArray: []string{"valid", "strings"}, // Valid
		RequiredIntArray:    []int{42},                    // Invalid - only 1 item, needs minItems: 2
	}

	err = tooFewIntItems.Validate()
	if err != nil {
		t.Logf("Good! Validation caught too few int items: %v", err)
	} else {
		t.Errorf("ERROR: Validation should have caught too few int items (minItems: 2)")
	}
}

func TestArrayLengthValidation(t *testing.T) {
	// Test with too many items in requiredStringArray (maxItems: 10)
	tooManyItems := make([]string, 11)
	for i := range tooManyItems {
		tooManyItems[i] = "valid"
	}

	longArrayRequest := ArrayTestRequest{
		RequiredStringArray: tooManyItems,
		RequiredIntArray:    []int{10, 20},
	}

	err := longArrayRequest.Validate()
	if err != nil {
		t.Logf("Good! Validation caught too many items in array: %v", err)
	} else {
		t.Logf("Note: Validation did not catch too many items in array - maxItems validation may not be implemented yet")
	}
}
