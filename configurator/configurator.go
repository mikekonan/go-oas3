package configurator

import (
	"context"

	"github.com/heetch/confita"
	"github.com/heetch/confita/backend/flags"
)

type Config struct {
	SwaggerAddr string `config:"swagger-addr,required"`
	PackagePath string `config:"package,required"`
	Path        string `config:"path,required"`

	ComponentsPackagePath string `config:"componentsPackage"`
	ComponentsPath        string `config:"componentsPath"`
}

func (config *Config) Defaults() *Config {
	config.SwaggerAddr = "swagger.yaml"

	return config
}

type Configurator struct {
	config *Config `di.inject:"config"`
}

func (configurator *Configurator) PostConstruct() (err error) {
	if err := confita.NewLoader(flags.NewBackend()).Load(context.Background(), configurator.config); err != nil {
		return err
	}

	if configurator.config.ComponentsPackagePath == "" {
		configurator.config.ComponentsPackagePath = configurator.config.PackagePath
	}

	if configurator.config.ComponentsPath == "" {
		configurator.config.ComponentsPath = configurator.config.Path
	}

	return nil
}
