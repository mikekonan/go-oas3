package example

import (
	"context"
	"github.com/go-chi/chi"
	"log"
	"net/http"
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

func (t transactionsService) DeleteTransactionsUUID(ctx context.Context, request DeleteTransactionsUUIDRequest) DeleteTransactionsUUIDResponse {
	log.Printf("processing delete transaction request - '%v'\n", request)
	if err := request.ProcessingResult.Err(); err != nil {
		return DeleteTransactionsUUIDResponseBuilder().
			StatusCode400().
			ApplicationJson().
			Body(GenericResponse{Result: GenericResponseResultEnumFailed}).
			Build()
	}

	log.Printf("deleting transaction - '%v'\n", request.Path.UUID)

	return DeleteTransactionsUUIDResponseBuilder().
		StatusCode200().
		ApplicationJson().
		Body(GenericResponse{Result: GenericResponseResultEnumSuccess}).
		Build()
}

type callbacksService struct{}

func (c callbacksService) PostCallbacksCallbackType(ctx context.Context, request PostCallbacksCallbackTypeRequest) PostCallbacksCallbackTypeResponse {
	return PostCallbacksCallbackTypeResponseBuilder().
		StatusCode200().
		Headers(PostCallbacksCallbackType200Headers{XJwsSignature: ""}).
		SetCookie(http.Cookie{
			Name:  "CookieName",
			Value: "CookieValue",
		}).ApplicationOctetStream().Body([]byte{}).Build()
}

func router() {
	router := chi.NewRouter()
	router.Route("/v1", func(r chi.Router) {
		TransactionsHandler(new(transactionsService), router, &Hooks{}, &securitySchemas{})
		CallbacksHandler(new(callbacksService), router, &Hooks{}, &securitySchemas{})
	})
}

type securitySchemas struct{}

func (self *securitySchemas) SecuritySchemeBearer(r *http.Request, scheme SecurityScheme, name string, value string) error {
	return nil
}

func (self *securitySchemas) SecuritySchemeBasic(r *http.Request, scheme SecurityScheme, name string, value string) error {
	return nil
}

func (self *securitySchemas) SecuritySchemeCookie(r *http.Request, scheme SecurityScheme, name string, value string) error {
	// value contains cookie's value, but it's still possible to get Cookie struct from request by its name
	cookie, _ := r.Cookie(name)
	if cookie != nil {
		log.Printf("Cookie domain: %s", cookie.Domain)
	}
	return nil
}
