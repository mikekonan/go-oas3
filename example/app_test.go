package example

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/mikekonan/go-types/country"
	"github.com/mikekonan/go-types/currency"
	"testing"
)

func TestValidation(t *testing.T) {

	type testType struct {
		Country  country.Alpha2Code
		Currency currency.Code
	}

	var body testType

	err := validation.ValidateStruct(&body,
		validation.Field(&body.Country, validation.Skip.When(body.Country == ""), validation.RuneLength(2, 2)),
		validation.Field(&body.Currency, validation.Skip.When(body.Currency == ""), validation.RuneLength(3, 3)))

	if err != nil {
		t.Fatal("must be no error on validation", err)
	}

	err = validation.ValidateStruct(&body,
		validation.Field(&body.Country, validation.RuneLength(2, 2)),
		validation.Field(&body.Currency, validation.RuneLength(3, 3)))

	if err == nil {
		t.Fatal("must be error on validation")
	}

	if err.Error() != "Country: '' is not valid ISO-3166-alpha2 code; Currency: '' is not valid ISO-4217 code." {
		t.Fatal("must be error: Country: '' is not valid ISO-3166-alpha2 code; Currency: '' is not valid ISO-4217 code.", err)
	}
}
