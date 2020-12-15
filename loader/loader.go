package loader

import (
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

	u, err := url.Parse(loader.config.SwaggerAddr)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "" && u.Host != "" {
		return swaggerLoader.LoadSwaggerFromURI(u)
	}

	return swaggerLoader.LoadSwaggerFromFile(loader.config.SwaggerAddr)
}
