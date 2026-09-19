//go:generate go run main.go --skipHandlers=false --skipStructs=false --skipTypes=false --skipPluginEvents=false --skipHookEvents=false
package main

import (
	"flag"
	"log"
	codegen "seanime/codegen/internal"
)

// Paths are relative to this directory, so codegen must be run from it — which is
// why the //go:generate directive above exists and why CI runs
// `go generate ./codegen` rather than `go run ./codegen`.
const (
	handlersSrcDir     = "../internal/handlers"
	internalSrcDir     = "../internal"
	eventsSrcDir       = "../internal/events"
	pluginEventsSrc    = "../internal/plugin/ui/events.go"
	pluginHookTypesDir = "../internal/extension_repo/goja_plugin_types"

	generatedDir      = "./generated"
	handlersJson      = "./generated/handlers.json"
	publicStructsJson = "./generated/public_structs.json"

	webApiOutDir         = "../seanime-web/src/api/generated"
	webPluginEventOutDir = "../seanime-web/src/app/(main)/_features/plugin/generated"
)

func main() {

	var skipHandlers bool
	flag.BoolVar(&skipHandlers, "skipHandlers", false, "Skip generating docs")

	var skipStructs bool
	flag.BoolVar(&skipStructs, "skipStructs", false, "Skip generating structs")

	var skipTypes bool
	flag.BoolVar(&skipTypes, "skipTypes", false, "Skip generating types")

	var skipPluginEvents bool
	flag.BoolVar(&skipPluginEvents, "skipPluginEvents", false, "Skip generating plugin events")

	var skipHookEvents bool
	flag.BoolVar(&skipHookEvents, "skipHookEvents", false, "Skip generating hook events")

	flag.Parse()

	if !skipHandlers {
		if err := codegen.GenerateHandlers(handlersSrcDir, generatedDir); err != nil {
			log.Fatalf("codegen: generating handlers: %v", err)
		}
	}

	if !skipStructs {
		if err := codegen.ExtractStructs(internalSrcDir, generatedDir); err != nil {
			log.Fatalf("codegen: extracting structs: %v", err)
		}
	}

	if !skipTypes {
		goStructStrs, err := codegen.GenerateTypescriptEndpointsFile(handlersJson, publicStructsJson, webApiOutDir, eventsSrcDir)
		if err != nil {
			log.Fatalf("codegen: generating TypeScript endpoints: %v", err)
		}
		if err := codegen.GenerateTypescriptFile(handlersJson, publicStructsJson, webApiOutDir, goStructStrs); err != nil {
			log.Fatalf("codegen: generating TypeScript types: %v", err)
		}
	}

	if !skipPluginEvents {
		if err := codegen.GeneratePluginEventFile(pluginEventsSrc, webPluginEventOutDir); err != nil {
			log.Fatalf("codegen: generating plugin events: %v", err)
		}
	}

	if !skipHookEvents {
		if err := codegen.GeneratePluginHooksDefinitionFile(pluginHookTypesDir, publicStructsJson, generatedDir); err != nil {
			log.Fatalf("codegen: generating plugin hook definitions: %v", err)
		}
	}

}
