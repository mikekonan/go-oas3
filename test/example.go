package test

import "github.com/google/uuid"

type PostCallbacksIntegrationTypeRequestBody struct {
	Field string `json:"field"`
}

type PostCallbacksIntegrationTypeRequestParameters struct {
	Header struct {
		XPaMerchantId uuid.UUID
	}

	Query struct {
		Selected bool
	}

	Path struct {
		UUID uuid.UUID
	}
}

type PostCallbacksIntegrationTypeRequest struct {
	Parameters  PostCallbacksIntegrationTypeRequestParameters
	RequestBody PostCallbacksIntegrationTypeRequestBody
}

type keka struct {
	Lol struct {
		Keka string
	}
}

type PostCallbacksIntegrationTypeRequestParams struct {
	Query struct {
		MerchantID  string
		OperationID uuid.UUID
	}
	Path struct {
		IntegrationType string
	}
}
