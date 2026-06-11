package example

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type transactionsService struct{}

func (t transactionsService) PostTransaction(ctx context.Context, request PostTransactionRequest) PostTransactionResponse {
	log.Printf("processing create transaction request...\n")

	if err := request.ProcessingResult.Err(); err != nil {
		return PostTransactionResponseBuilder().
			StatusCode400().
			ApplicationJson().
			Body(GenericResponse{Result: GenericResponseResultEnumFailed}).
			Build()
	}

	log.Printf("creating transaction - '%v'\n", request.Body)

	res := GenericResponse{Result: GenericResponseResultEnumSuccess}
	if err := res.Validate(); err != nil {
		return PostTransactionResponseBuilder().
			StatusCode500().
			ApplicationJson().
			Body(GenericResponse{Result: GenericResponseResultEnumFailed}).
			Build()
	}

	return PostTransactionResponseBuilder().
		StatusCode201().
		ApplicationJson().
		Body(res).
		Build()
}

func (t transactionsService) PutTransaction(ctx context.Context, request PutTransactionRequest) PutTransactionResponse {
	log.Printf("processing update transaction request...\n")

	if err := request.ProcessingResult.Err(); err != nil {
		return PutTransactionResponseBuilder().
			StatusCode400().
			ApplicationJson().
			Body(GenericResponse{Result: GenericResponseResultEnumFailed}).
			Build()
	}

	log.Printf("updating transaction - '%v'\n", request.Body)

	res := GenericResponse{Result: GenericResponseResultEnumSuccess}
	if err := res.Validate(); err != nil {
		return PutTransactionResponseBuilder().
			StatusCode500().
			ApplicationJson().
			Body(GenericResponse{Result: GenericResponseResultEnumFailed}).
			Build()
	}

	return PutTransactionResponseBuilder().
		StatusCode200().
		ApplicationJson().
		Body(res).
		Build()
}

func (t transactionsService) DeleteTransactionsUUID(ctx context.Context, request DeleteTransactionsUUIDRequest) DeleteTransactionsUUIDResponse {
	log.Printf("processing delete transaction request...\n")

	if err := request.ProcessingResult.Err(); err != nil {
		return DeleteTransactionsUUIDResponseBuilder().
			StatusCode400().
			ApplicationJson().
			Body(GenericResponse{Result: GenericResponseResultEnumFailed}).
			Build()
	}

	log.Printf("deleting transaction with UUID: %s\n", request.Path.UUID)

	// The 200 response declares a Content-Encoding header, which the generator
	// treats specially: it's plumbed via BodyBytesWithEncoding rather than the
	// per-response Headers struct. For an ordinary in-process body we just
	// emit the JSON without compression and skip the encoding header.
	return DeleteTransactionsUUIDResponseBuilder().
		StatusCode200().
		ApplicationJson().
		Body(GenericResponse{Result: GenericResponseResultEnumSuccess}).
		Build()
}

type authService struct{}

func (a authService) GetSecureEndpoint(ctx context.Context, request GetSecureEndpointRequest) GetSecureEndpointResponse {
	return GetSecureEndpointResponseBuilder().
		StatusCode200().
		ApplicationJson().
		Body(GetSecureEndpoint200ApplicationJson{Message: "Hello from secure endpoint"}).
		Build()
}

func (a authService) GetSemiSecureEndpoint(ctx context.Context, request GetSemiSecureEndpointRequest) GetSemiSecureEndpointResponse {
	return GetSemiSecureEndpointResponseBuilder().
		StatusCode200().
		ApplicationJson().
		Body(GetSemiSecureEndpoint200ApplicationJson{
			Message: "Hello from semi-secure endpoint",
			ApiKey:  "received",
		}).
		Build()
}

func (a authService) PostBearerEndpoint(ctx context.Context, request PostBearerEndpointRequest) PostBearerEndpointResponse {
	return PostBearerEndpointResponseBuilder().
		StatusCode200().
		ApplicationJson().
		Body(PostBearerEndpoint200ApplicationJson{Message: "Hello from bearer endpoint"}).
		Build()
}

type callbacksService struct{}

func (c callbacksService) PostCallbacksCallbackType(ctx context.Context, request PostCallbacksCallbackTypeRequest) PostCallbacksCallbackTypeResponse {
	log.Printf("processing callback of type: %s\n", request.Path.CallbackType)

	return PostCallbacksCallbackTypeResponseBuilder().
		StatusCode200().
		Headers(PostCallbacksCallbackType200Headers{XJwsSignature: "example-signature"}).
		SetCookie(http.Cookie{
			Name:     "JSESSIONID",
			Value:    "example123",
			Path:     "/",
			HttpOnly: true,
		}).
		ApplicationOctetStream().
		Body(request.Body). // echo back the raw payload
		Build()
}

type securitySchemas struct{}

func (securitySchemas) SecuritySchemeBearer(r *http.Request, scheme SecurityScheme, name, value string) error {
	return nil
}

func (securitySchemas) SecuritySchemeApiKeyAuth(r *http.Request, scheme SecurityScheme, name, value string) error {
	return nil
}

func (securitySchemas) SecuritySchemeBasic(r *http.Request, scheme SecurityScheme, name, value string) error {
	return nil
}

func (securitySchemas) SecuritySchemeCookie(r *http.Request, scheme SecurityScheme, name, value string) error {
	return nil
}

// NewApp wires the example service implementations into a chi router via the
// generated Handler constructors. Mirrors the pattern recommended in README
// for production use.
func NewApp() *http.Server {
	r := chi.NewRouter()

	schemas := securitySchemas{}
	TransactionsHandler(transactionsService{}, r, nil, schemas)
	AuthHandler(authService{}, r, nil, schemas)
	CallbacksHandler(callbacksService{}, r, nil, schemas)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return &http.Server{
		Addr:    ":8080",
		Handler: r,
	}
}
