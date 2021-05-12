package configurator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/heetch/confita"
	"github.com/heetch/confita/backend/flags"
)

type Config struct {
	SwaggerAddr string `config:"swagger-addr,required"`
	Package     string `config:"package,required"`
	Path        string `config:"path,required"`

	ComponentsPackage string `config:"componentsPackage"`
	ComponentsPath    string `config:"componentsPath"`

	Authorization string `config:"authorization,short=a,description=a list of comma-separated key:value pairs to be sent as headers alongside each http request"`
}

func (config *Config) Defaults() *Config {
	config.SwaggerAddr = "swagger.yaml"

	return config
}

func (config *Config) Headers() (http.Header, error) {
	headers := strings.Split(config.Authorization, ",")
	out := make(http.Header)

	for _, header := range headers {
		keyValue := strings.Split(header, ":")
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("invalid header format: %q", header)
		}

		out[keyValue[0]] = []string{keyValue[1]}
	}

	return out, nil
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

	if configurator.config.Package, err = configurator.concatPaths(configurator.config.Path); err != nil {
		return err
	}

	if configurator.config.ComponentsPackage == "" {
		configurator.config.ComponentsPackage = configurator.config.Package
	}

	if configurator.config.ComponentsPath == "" {
		configurator.config.ComponentsPath = configurator.config.Path
	}

	return nil
}
