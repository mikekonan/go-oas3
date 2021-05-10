package loader

import (
	"net/http"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/mikekonan/go-oas3/configurator"
)

type Loader struct {
	config *configurator.Config `di.inject:"config"`
}

func (loader *Loader) Load() (*openapi3.Swagger, error) {
	swaggerLoader := openapi3.NewSwaggerLoader()
	swaggerLoader.IsExternalRefsAllowed = true

	if loader.config.Authorization != "" {
		headers, err := loader.config.Headers()
		if err != nil {
			return nil, err
		}

		setTransportWithHeaders(headers)
	}

	u, err := url.Parse(loader.config.SwaggerAddr)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "" && u.Host != "" {
		return swaggerLoader.LoadSwaggerFromURI(u)
	}

	return swaggerLoader.LoadSwaggerFromFile(loader.config.SwaggerAddr)
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setTransportWithHeaders(headers http.Header) {
	http.DefaultClient.Transport = RoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		for key, value := range headers {
			request.Header[key] = value
		}

		return http.DefaultTransport.RoundTrip(request)
	})
}
