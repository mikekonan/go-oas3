package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

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
	cfg := new(configurator.Config).Defaults()
	if err := configurator.New(cfg).LoadFlags(); err != nil {
		log.Fatal(err.Error())
	}

	normalizer := generator.NewNormalizer()
	typeFiller := generator.NewType(normalizer, cfg)
	app := application.New(
		loader.New(cfg),
		generator.New(normalizer, typeFiller, cfg),
		writer.New(cfg),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err.Error())
	}
}
