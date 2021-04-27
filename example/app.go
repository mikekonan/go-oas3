package example

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi"
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

	return PostTransactionResponseBuilder().
		StatusCode201().
		ApplicationJson().
		Body(GenericResponse{Result: GenericResponseResultEnumSuccess}).
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

func router() {
	router := chi.NewRouter()
	handler := TransactionsHandler(new(transactionsService), router, &Hooks{})

	http.Handle("v1", handler)
}
