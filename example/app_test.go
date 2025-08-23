package example

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/mikekonan/go-types/v2/country"
	"github.com/mikekonan/go-types/v2/currency"
)

func TestValidation(t *testing.T) {
	type testType struct {
		Country  country.Alpha2Code
		Currency currency.Code
	}

	var body testType

	// Test validation with empty fields (should pass with Skip)
	err := validation.ValidateStruct(&body,
		validation.Field(&body.Country, validation.Skip.When(body.Country == ""), validation.RuneLength(2, 2)),
		validation.Field(&body.Currency, validation.Skip.When(body.Currency == ""), validation.RuneLength(3, 3)))

	if err != nil {
		t.Fatal("must be no error on validation with empty fields", err)
	}

	// Test validation without Skip (should fail for empty fields)
	err = validation.ValidateStruct(&body,
		validation.Field(&body.Country, validation.RuneLength(2, 2)),
		validation.Field(&body.Currency, validation.RuneLength(3, 3)))

	if err == nil {
		t.Fatal("must be error on validation with empty required fields")
	}

	// Test with valid values
	body.Country = "US"
	body.Currency = "USD"

	err = validation.ValidateStruct(&body,
		validation.Field(&body.Country, validation.RuneLength(2, 2)),
		validation.Field(&body.Currency, validation.RuneLength(3, 3)))

	if err != nil {
		t.Fatal("must be no error on validation with valid fields", err)
	}
}

func TestGenericResponse(t *testing.T) {
	// Test valid enum values - note: GenericResponse.Result is of type Result enum
	validResponses := []GenericResponse{
		{Result: ResultSuccess},
		{Result: ResultFailed},
	}

	for _, resp := range validResponses {
		if err := resp.Validate(); err != nil {
			t.Errorf("expected valid response %v to pass validation, got error: %v", resp, err)
		}
	}

	// Test Result enum validation directly since GenericResponse.Validate() returns nil
	// Valid enum values should validate correctly
	validResults := []Result{
		ResultSuccess,
		ResultFailed,
	}

	for _, result := range validResults {
		if err := result.Validate(); err != nil {
			t.Errorf("expected valid result %v to pass validation, got error: %v", result, err)
		}
	}

	// Test invalid enum value
	invalidResult := Result("invalid")
	if err := invalidResult.Validate(); err == nil {
		t.Error("expected invalid result to fail validation")
	}
}

func TestCreateTransactionRequest(t *testing.T) {
	// Test valid request
	validRequest := CreateTransactionRequest{
		Description: "Test transaction description that meets minimum length",
		Title:       "Test Title",
		Amount:      10.50,
		Currency:    "USD",
	}

	if err := validRequest.Validate(); err != nil {
		t.Errorf("expected valid request to pass validation, got error: %v", err)
	}

	// Test invalid request - description too short
	invalidRequest := CreateTransactionRequest{
		Description: "Short", // Too short (minimum 8 chars)
		Title:       "Test Title",
		Amount:      10.50,
		Currency:    "USD",
	}

	if err := invalidRequest.Validate(); err == nil {
		t.Error("expected request with short description to fail validation")
	}

	// Test invalid request - amount too small
	invalidAmountRequest := CreateTransactionRequest{
		Description: "Test transaction description that meets minimum length",
		Title:       "Test Title", 
		Amount:      0.005, // Below minimum exclusive 0.009
		Currency:    "USD",
	}

	if err := invalidAmountRequest.Validate(); err == nil {
		t.Error("expected request with small amount to fail validation")
	}
}