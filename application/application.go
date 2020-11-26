package application

import (
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/mikekonan/go-oas3/loader"
	"github.com/mikekonan/go-oas3/transformer"
)

type Application struct {
	config      *configurator.Config     `di.inject:"config"`
	loader      *loader.Loader           `di.inject:"loader"`
	transformer *transformer.Transformer `di.inject:"transformer"`
}

func (app *Application) Run() error {
	swagger, err := app.loader.Load(app.config.SwaggerAddr)

	if err != nil {
		return err
	}

	app.transformer.Transform(swagger)

	return nil
}
