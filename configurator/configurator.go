package configurator

import (
	"context"
	"os"
	"path"

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

func (config *Config) IsComponentsPathDiffersFromWrappersPath() bool {
	return config.ComponentsPackagePath != config.PackagePath
}

type Configurator struct {
	config *Config `di.inject:"config"`
}

func (configurator *Configurator) concatPaths(filePath string) (string, error) {
	if filePath[0] == '.' {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}

		return path.Join(wd, filePath), nil
	}

	return filePath, nil
}

func (configurator *Configurator) PostConstruct() (err error) {
	if err := confita.NewLoader(flags.NewBackend()).Load(context.Background(), configurator.config); err != nil {
		return err
	}

	if configurator.config.Path, err = configurator.concatPaths(configurator.config.Path); err != nil {
		return err
	}

	if configurator.config.PackagePath, err = configurator.concatPaths(configurator.config.Path); err != nil {
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
