package example

import (
	"context"
	"net/http"

	"github.com/go-chi/chi"
)

type petsService struct {
}

func (p petsService) GetPets(ctx context.Context, request GetPetsRequest) GetPetsResponse {
	builder := GetPetsResponseBuilder()

	if request.ProcessingResult.Type() != ParseSucceed {
		//assume that you have described affordable status code in your openapi spec
	}

	//parse query params
	if request.Query.Limit == 0 {
		//
	}

	return builder.StatusCode200().Headers(GetPets200Headers{XNext: "next"}).ApplicationJson().Body(Pets{}).Build()
}

func (p petsService) PostPets(ctx context.Context, request PostPetsRequest) PostPetsResponse {
	builder := PostPetsResponseBuilder()

	if request.ProcessingResult.Type() != ParseSucceed {
		//assume that you have described affordable status code in your openapi spec
	}

	return builder.StatusCode201().Build()
}

func (p petsService) GetPetsPetID(ctx context.Context, request GetPetsPetIDRequest) GetPetsPetIDResponse {
	if request.ProcessingResult.Type() != ParseSucceed {
		//assume that you have described affordable status code in your openapi spec
	}

	//pet := store.getPet(request.Path.PetID)
	return GetPetsPetIDResponseBuilder().StatusCode200().ApplicationJson().Body(Pet{}).Build()
}

func router() {
	router := chi.NewRouter()
	handler := PetsHandler(new(petsService), router, &Hooks{
		RequestBodyUnmarshalFailed:    nil,
		RequestHeaderParseFailed:      nil,
		RequestPathParseFailed:        nil,
		RequestQueryParseFailed:       nil,
		RequestBodyUnmarshalCompleted: nil,
		RequestHeaderParseCompleted:   nil,
		RequestPathParseCompleted:     nil,
		RequestQueryParseCompleted:    nil,
		RequestParseCompleted:         nil,
		RequestProcessingCompleted:    nil,
		RequestRedirectStarted:        nil,
		ResponseBodyMarshalCompleted:  nil,
		ResponseBodyWriteCompleted:    nil,
		ResponseBodyMarshalFailed:     nil,
		ResponseBodyWriteFailed:       nil,
		ServiceCompleted:              nil,
	})

	http.Handle("v1", handler)
}
