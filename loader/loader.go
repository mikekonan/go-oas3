package loader

import (
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
)

type Loader struct{}

func (*Loader) Load(path string) (*openapi3.Swagger, error) {
	loader := openapi3.NewSwaggerLoader()
	loader.IsExternalRefsAllowed = true

	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "" && u.Host != "" {
		return loader.LoadSwaggerFromURI(u)
	}

	return loader.LoadSwaggerFromFile(path)
}
