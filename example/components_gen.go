// This file is generated by github.com/mikekonan/go-oas3. DO NOT EDIT.

package example

import (
	"encoding/json"
	"fmt"
	uuid "github.com/google/uuid"
	countries "github.com/mikekonan/go-types/country"
	currency "github.com/mikekonan/go-types/currency"
	email "github.com/mikekonan/go-types/email"
	url "github.com/mikekonan/go-types/url"
	"regexp"
)

var createTransactionRequestRegexParamRegex = regexp.MustCompile("^[.?\\d]+$")

type URL = url.URL

type createTransactionRequest struct {
	CallbackURL   url.URL              `json:"callbackURL"`
	Country       countries.Alpha2Code `json:"country"`
	Currency      currency.Code        `json:"currency"`
	Email         email.Email          `json:"email"`
	RegexParam    string               `json:"regexParam"`
	TransactionID uuid.UUID            `json:"transactionID"`
}

type CreateTransactionRequest struct {
	CallbackURL   url.URL              `json:"callbackURL"`
	Country       countries.Alpha2Code `json:"country"`
	Currency      currency.Code        `json:"currency"`
	Email         email.Email          `json:"email"`
	RegexParam    string               `json:"regexParam"`
	TransactionID uuid.UUID            `json:"transactionID"`
}

func (body *CreateTransactionRequest) UnmarshalJSON(data []byte) error {
	var value createTransactionRequest
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	body.Currency = value.Currency
	body.Email = value.Email
	if !createTransactionRequestRegexParamRegex.MatchString(body.RegexParam) {
		return fmt.Errorf("RegexParam not matched by the '^[.?\\d]+$' regex")
	}

	body.RegexParam = value.RegexParam
	body.TransactionID = value.TransactionID
	body.CallbackURL = value.CallbackURL
	body.Country = value.Country

	return nil
}

type Email = email.Email

type genericResponse struct {
	Result GenericResponseResultEnum `json:"result"`
}

type GenericResponse struct {
	Result GenericResponseResultEnum `json:"result"`
}

func (body *GenericResponse) UnmarshalJSON(data []byte) error {
	var value genericResponse
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	body.Result = value.Result

	return nil
}

type GenericResponseResultEnum string

var GenericResponseResultEnumSuccess GenericResponseResultEnum = "success"
var GenericResponseResultEnumFailed GenericResponseResultEnum = "failed"

func (enum GenericResponseResultEnum) Check() error {
	switch enum {
	case GenericResponseResultEnumSuccess, GenericResponseResultEnumFailed:

		return nil
	}

	return fmt.Errorf("invalid GenericResponseResultEnum enum value")
}

func (enum *GenericResponseResultEnum) UnmarshalJSON(data []byte) error {
	var strValue string
	if err := json.Unmarshal(data, &strValue); err != nil {

		return err
	}
	enumValue := GenericResponseResultEnum(strValue)
	if err := enumValue.Check(); err != nil {

		return err
	}
	*enum = enumValue

	return nil
}
