package application

import (
	"github.com/mikekonan/go-oas3/generator"
	"github.com/mikekonan/go-oas3/loader"
	"github.com/mikekonan/go-oas3/writer"
)

type Application struct {
	loader    *loader.Loader
	generator *generator.Generator
	writer    *writer.Writer
}

func New(l *loader.Loader, g *generator.Generator, w *writer.Writer) *Application {
	return &Application{loader: l, generator: g, writer: w}
}

func (app *Application) Run() error {
	swagger, err := app.loader.Load()

	if err != nil {
		return err
	}

	return app.writer.Write(app.generator.Generate(swagger))
}
