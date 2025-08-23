package example

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type transactionsService struct{}

func (t transactionsService) PostTransaction(ctx context.Context, request *PostTransactionRequest) *PostTransactionResponse {
	log.Printf("processing create transaction request...\n")

	if err := request.ProcessingResult.Err(); err != nil {
		return &PostTransactionResponse{
			response: &response{
				statusCode: 400,
				body:       GenericResponse{Result: ResultFailed},
				headers:    map[string]string{"Content-Type": "application/json"},
			},
		}
	}

	log.Printf("creating transaction - '%v'\n", request.Body)

	res := GenericResponse{Result: ResultSuccess}
	if err := res.Validate(); err != nil {
		return &PostTransactionResponse{
			response: &response{
				statusCode: 500,
				body:       GenericResponse{Result: ResultFailed},
				headers:    map[string]string{"Content-Type": "application/json"},
			},
		}
	}

	return &PostTransactionResponse{
		response: &response{
			statusCode: 201,
			body:       res,
			headers:    map[string]string{"Content-Type": "application/json"},
		},
	}
}

func (t transactionsService) PutTransaction(ctx context.Context, request *PutTransactionRequest) *PutTransactionResponse {
	log.Printf("processing update transaction request...\n")

	if err := request.ProcessingResult.Err(); err != nil {
		return &PutTransactionResponse{
			response: &response{
				statusCode: 400,
				body:       GenericResponse{Result: ResultFailed},
				headers:    map[string]string{"Content-Type": "application/json"},
			},
		}
	}

	log.Printf("updating transaction - '%v'\n", request.Body)

	res := GenericResponse{Result: ResultSuccess}
	if err := res.Validate(); err != nil {
		return &PutTransactionResponse{
			response: &response{
				statusCode: 500,
				body:       GenericResponse{Result: ResultFailed},
				headers:    map[string]string{"Content-Type": "application/json"},
			},
		}
	}

	return &PutTransactionResponse{
		response: &response{
			statusCode: 200,
			body:       res,
			headers:    map[string]string{"Content-Type": "application/json"},
		},
	}
}

func (t transactionsService) DeleteTransactionsUUID(ctx context.Context, request *DeleteTransactionsUUIDRequest) *DeleteTransactionsUUIDResponse {
	log.Printf("processing delete transaction request...\n")

	if err := request.ProcessingResult.Err(); err != nil {
		return &DeleteTransactionsUUIDResponse{
			response: &response{
				statusCode: 400,
				body:       GenericResponse{Result: ResultFailed},
				headers:    map[string]string{"Content-Type": "application/json"},
			},
		}
	}

	log.Printf("deleting transaction with UUID: %s\n", request.path.UUID)

	res := GenericResponse{Result: ResultSuccess}
	return &DeleteTransactionsUUIDResponse{
		response: &response{
			statusCode: 200,
			body:       res,
			headers: map[string]string{
				"Content-Type":     "application/json",
				"Content-Encoding": "gzip",
			},
		},
	}
}

type authService struct{}

func (a authService) GetSecureEndpoint(ctx context.Context, request *GetSecureEndpointRequest) *GetSecureEndpointResponse {
	return &GetSecureEndpointResponse{
		response: &response{
			statusCode: 200,
			body:       map[string]string{"message": "Hello from secure endpoint"},
			headers:    map[string]string{"Content-Type": "application/json"},
		},
	}
}

func (a authService) GetSemiSecureEndpoint(ctx context.Context, request *GetSemiSecureEndpointRequest) *GetSemiSecureEndpointResponse {
	return &GetSemiSecureEndpointResponse{
		response: &response{
			statusCode: 200,
			body: map[string]string{
				"message": "Hello from semi-secure endpoint",
				"apiKey":  "received",
			},
			headers: map[string]string{"Content-Type": "application/json"},
		},
	}
}

func (a authService) PostBearerEndpoint(ctx context.Context, request *PostBearerEndpointRequest) *PostBearerEndpointResponse {
	return &PostBearerEndpointResponse{
		response: &response{
			statusCode: 200,
			body:       map[string]string{"message": "Hello from bearer endpoint"},
			headers:    map[string]string{"Content-Type": "application/json"},
		},
	}
}

type callbacksService struct{}

func (c callbacksService) PostCallbacksCallbackType(ctx context.Context, request *PostCallbacksCallbackTypeRequest) *PostCallbacksCallbackTypeResponse {
	log.Printf("processing callback of type: %s\n", request.path.CallbackType)
	
	return &PostCallbacksCallbackTypeResponse{
		response: &response{
			statusCode: 200,
			body:       request.Body, // Echo back the raw payload
			headers: map[string]string{
				"Content-Type":     "application/octet-stream",
				"Set-Cookie":       "JSESSIONID=example123; Path=/; HttpOnly",
				"x-jws-signature":  "example-signature",
			},
		},
	}
}

func NewApp() *http.Server {
	// Create services
	transactionsService := &transactionsService{}
	authService := &authService{}
	callbacksService := &callbacksService{}

	// Create routers
	transactionsRouter := NewTransactionsRouter(transactionsService)
	authRouter := NewAuthRouter(authService)
	callbacksRouter := NewCallbacksRouter(callbacksService)

	// Main router
	mainRouter := chi.NewRouter()

	// Manually register routes (since generated code doesn't auto-register)
	// Transaction routes
	mainRouter.Post("/transaction", func(w http.ResponseWriter, r *http.Request) {
		transactionsRouter.postTransaction(w, r)
	})
	mainRouter.Put("/transaction", func(w http.ResponseWriter, r *http.Request) {
		transactionsRouter.putTransaction(w, r)
	})
	mainRouter.Delete("/transactions/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		transactionsRouter.deleteTransactionsUUID(w, r)
	})

	// Auth routes  
	mainRouter.Get("/secure-endpoint", func(w http.ResponseWriter, r *http.Request) {
		authRouter.getSecureEndpoint(w, r)
	})
	mainRouter.Get("/semi-secure-endpoint", func(w http.ResponseWriter, r *http.Request) {
		authRouter.getSemiSecureEndpoint(w, r)
	})
	mainRouter.Post("/bearer-endpoint", func(w http.ResponseWriter, r *http.Request) {
		authRouter.postBearerEndpoint(w, r)
	})

	// Callback routes
	mainRouter.Post("/callbacks/{callbackType}", func(w http.ResponseWriter, r *http.Request) {
		callbacksRouter.postCallbacksCallbackType(w, r)
	})

	// Health check endpoint
	mainRouter.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	return &http.Server{
		Addr:    ":8080",
		Handler: mainRouter,
	}
}