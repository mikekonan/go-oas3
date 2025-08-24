package arraytest

import (
	"testing"
)

func TestDebugRequiredStringArrayValidation(t *testing.T) {
	// Test just RequiredStringArray with empty array
	req := ArrayTestRequest{
		RequiredStringArray: []string{}, // Empty - should fail minItems: 1
		RequiredIntArray:    []int{10, 20}, // Valid
	}
	
	err := req.Validate()
	t.Logf("Empty RequiredStringArray validation result: %v", err)
	
	// Test with one item (should pass)
	req2 := ArrayTestRequest{
		RequiredStringArray: []string{"test"}, // 1 item - should pass minItems: 1
		RequiredIntArray:    []int{10, 20}, // Valid
	}
	
	err2 := req2.Validate()
	t.Logf("One item RequiredStringArray validation result: %v", err2)
	
	// Test with too many items (maxItems: 10)
	manyItems := make([]string, 11)
	for i := range manyItems {
		manyItems[i] = "test"
	}
	req3 := ArrayTestRequest{
		RequiredStringArray: manyItems, // 11 items - should fail maxItems: 10
		RequiredIntArray:    []int{10, 20}, // Valid
	}
	
	err3 := req3.Validate()
	t.Logf("Too many RequiredStringArray validation result: %v", err3)
}