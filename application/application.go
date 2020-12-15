package application

import (
	"github.com/mikekonan/go-oas3/generator"
	"github.com/mikekonan/go-oas3/loader"
	"github.com/mikekonan/go-oas3/writer"
)

type Application struct {
	loader    *loader.Loader       `di.inject:"loader"`
	generator *generator.Generator `di.inject:"generator"`
	writer    *writer.Writer       `di.inject:"writer"`
}

func (app *Application) Run() error {
	swagger, err := app.loader.Load()

	if err != nil {
		return err
	}

	return app.writer.Write(app.generator.Generate(swagger))
}
