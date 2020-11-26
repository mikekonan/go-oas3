package main

import (
	"log"
	"reflect"

	"github.com/goioc/di"

	"github.com/mikekonan/go-oas3/application"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/mikekonan/go-oas3/loader"
	"github.com/mikekonan/go-oas3/transformer"
)

func main() {
	_, _ = di.RegisterBeanInstance("config", new(configurator.Config).Defaults())
	_, _ = di.RegisterBean("loader", reflect.TypeOf((*loader.Loader)(nil)))
	_, _ = di.RegisterBean("configurator", reflect.TypeOf((*configurator.Configurator)(nil)))
	_, _ = di.RegisterBean("transformer", reflect.TypeOf((*transformer.Transformer)(nil)))
	_, _ = di.RegisterBean("app", reflect.TypeOf((*application.Application)(nil)))

	if err := di.InitializeContainer(); err != nil {
		log.Fatal(err.Error())
	}

	app := di.GetInstance("app").(*application.Application)

	if err := app.Run(); err != nil {
		log.Fatal(err.Error())
	}
}
