package configurator

import (
	"context"

	"github.com/heetch/confita"
	"github.com/heetch/confita/backend/flags"
)

type Config struct {
	SwaggerAddr     string `config:"swagger-addr,required"`
	Package         string `config:"package,required"`
	Destination     string `config:"destination,required"`
	SegregateByTags bool   `config:"segregate"`
}

func (config *Config) Defaults() *Config {
	config.Package = "oas"
	config.Destination = "oas"
	config.SegregateByTags = false
	config.SwaggerAddr = "swagger.yaml"

	return config
}

type Configurator struct {
	config *Config `di.inject:"config"`
}

func (configurator *Configurator) PostConstruct() (err error) {
	return confita.NewLoader(flags.NewBackend()).Load(context.Background(), configurator.config)
}
