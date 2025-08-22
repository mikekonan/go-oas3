package main

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime/debug"

	"github.com/goioc/di"

	"github.com/mikekonan/go-oas3/application"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/mikekonan/go-oas3/generator"
	"github.com/mikekonan/go-oas3/loader"
	"github.com/mikekonan/go-oas3/writer"
)

func getVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	
	// Get module version
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	
	// Fallback to VCS revision if available
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			if len(setting.Value) >= 7 {
				return "dev-" + setting.Value[:7]
			}
			return "dev-" + setting.Value
		}
	}
	
	return "development"
}

func main() {
	// Check for version flag before confita processes flags
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" {
			fmt.Printf("go-oas3 version %s\n", getVersion())
			
			// Print additional build info
			if info, ok := debug.ReadBuildInfo(); ok {
				fmt.Printf("Go version: %s\n", info.GoVersion)
				for _, setting := range info.Settings {
					switch setting.Key {
					case "vcs.revision":
						fmt.Printf("Git commit: %s\n", setting.Value)
					case "vcs.time":
						fmt.Printf("Build time: %s\n", setting.Value)
					case "vcs.modified":
						if setting.Value == "true" {
							fmt.Printf("Modified: true (development build)\n")
						}
					}
				}
			}
			os.Exit(0)
		}
	}

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
