package main

import (
	"log"
	"reflect"

	"github.com/goioc/di"

	"github.com/mikekonan/go-oas3/application"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/mikekonan/go-oas3/generator"
	"github.com/mikekonan/go-oas3/loader"
	"github.com/mikekonan/go-oas3/writer"
)

func main() {
	di.RegisterBeanInstance("config", new(configurator.Config).Defaults())
	di.RegisterBean("loader", reflect.TypeOf((*loader.Loader)(nil)))
	di.RegisterBean("configurator", reflect.TypeOf((*configurator.Configurator)(nil)))
	di.RegisterBean("generator", reflect.TypeOf((*generator.Generator)(nil)))
	di.RegisterBean("typeFiller", reflect.TypeOf((*generator.Type)(nil)))
	di.RegisterBean("normalizer", reflect.TypeOf((*generator.Normalizer)(nil)))
	di.RegisterBean("writer", reflect.TypeOf((*writer.Writer)(nil)))
	di.RegisterBean("app", reflect.TypeOf((*application.Application)(nil)))

	if err := di.InitializeContainer(); err != nil {
		log.Fatal(err.Error())
	}

	app := di.GetInstance("app").(*application.Application)

	if err := app.Run(); err != nil {
		log.Fatal(err.Error())
	}
}
