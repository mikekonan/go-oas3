package generic

import (
	"context"
	"net/http"
)

type widgetsService struct{}

// PostClassicWidgets uses the classic per-operation builder cascade.
func (widgetsService) PostClassicWidgets(_ context.Context, req PostClassicWidgetsRequest) PostClassicWidgetsResponse {
	if err := req.ProcessingResult.Err(); err != nil {
		return PostClassicWidgetsResponseBuilder().
			StatusCode400().
			ApplicationJson().
			Body(ErrorBody{Code: "bad_request", Message: err.Error()}).
			Build()
	}

	return PostClassicWidgetsResponseBuilder().
		StatusCode200().
		ApplicationJson().
		Body(Widget{ID: "classic-1", Name: req.Body.Name}).
		Build()
}

// PostWidgets uses the generic Response[B] / ResponseBuilder[B] pattern.
func (widgetsService) PostWidgets(_ context.Context, req PostWidgetsRequest) *Response[Widget] {
	if err := req.ProcessingResult.Err(); err != nil {
		return NewResponse[Widget]().
			Status(400).
			BodyAny(ErrorBody{Code: "bad_request", Message: err.Error()}).
			Build()
	}

	return NewResponse[Widget]().
		Status(200).
		Body(Widget{ID: "generic-1", Name: req.Body.Name}).
		Build()
}

// GetWidgetsEcho exercises custom headers + Set-Cookie via the generic builder.
func (widgetsService) GetWidgetsEcho(_ context.Context, _ GetWidgetsEchoRequest) *Response[Widget] {
	return NewResponse[Widget]().
		Status(200).
		Header("X-Request-ID", "req-abc-123").
		Cookie(http.Cookie{Name: "widget_session", Value: "tok-42", Path: "/"}).
		Body(Widget{ID: "echo-1", Name: "echoed"}).
		Build()
}

// GetWidgetsMulti exercises content-type negotiation via the typed
// ApplicationXml() helper.
func (widgetsService) GetWidgetsMulti(_ context.Context, _ GetWidgetsMultiRequest) *Response[Widget] {
	return NewResponse[Widget]().
		Status(200).
		ApplicationXml().
		Body(Widget{ID: "multi-1", Name: "xml-body"}).
		Build()
}

// GetWidgetsRedirect returns a 302 with a Location header.
func (widgetsService) GetWidgetsRedirect(_ context.Context, _ GetWidgetsRedirectRequest) *Response[any] {
	return NewResponse[any]().
		Status(302).
		Redirect("/widgets/echo").
		Build()
}

// GetWidgetsSecure requires a Bearer token to have been validated upstream.
// The parser returns 401 path automatically if security failed.
func (widgetsService) GetWidgetsSecure(_ context.Context, req GetWidgetsSecureRequest) *Response[Widget] {
	if req.ProcessingResult.Type() == SecurityParseFailed || req.ProcessingResult.Type() == SecurityCheckFailed {
		return NewResponse[Widget]().
			Status(401).
			BodyAny(ErrorBody{Code: "unauthorized", Message: "auth failed"}).
			Build()
	}
	return NewResponse[Widget]().
		Status(200).
		Body(Widget{ID: "secure-1", Name: "secret"}).
		Build()
}

// widgetsSchemas implements the generated SecuritySchemas interface.
// SecuritySchemeBearer validates if the token equals "valid-token".
type widgetsSchemas struct{}

func (widgetsSchemas) SecuritySchemeBearer(_ *http.Request, _ SecurityScheme, _, value string) error {
	if value != "valid-token" {
		return errInvalidToken
	}
	return nil
}

var errInvalidToken = &authError{"invalid token"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }
