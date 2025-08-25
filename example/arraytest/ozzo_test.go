package arraytest

import (
	"testing"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

func TestOzzoValidationLength(t *testing.T) {
	// Test ozzo-validation Length directly
	type TestStruct struct {
		Items []string
	}
	
	// Test empty array with Length(1, 10)
	s1 := TestStruct{Items: []string{}}
	err1 := validation.ValidateStruct(&s1,
		validation.Field(&s1.Items, validation.Length(1, 10)),
	)
	t.Logf("Empty array with Length(1, 10): %v", err1)
	
	// Test nil array with Length(1, 10)
	s2 := TestStruct{Items: nil}
	err2 := validation.ValidateStruct(&s2,
		validation.Field(&s2.Items, validation.Length(1, 10)),
	)
	t.Logf("Nil array with Length(1, 10): %v", err2)
	
	// Test one item with Length(1, 10) 
	s3 := TestStruct{Items: []string{"test"}}
	err3 := validation.ValidateStruct(&s3,
		validation.Field(&s3.Items, validation.Length(1, 10)),
	)
	t.Logf("One item with Length(1, 10): %v", err3)
	
	// Test empty array with Required + Length(1, 10)
	s4 := TestStruct{Items: []string{}}
	err4 := validation.ValidateStruct(&s4,
		validation.Field(&s4.Items, validation.Required, validation.Length(1, 10)),
	)
	t.Logf("Empty array with Required + Length(1, 10): %v", err4)
}