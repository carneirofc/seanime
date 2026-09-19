package codegen

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadHandlers reads the handlers.json produced by GenerateHandlers.
func LoadHandlers(path string) ([]*RouteHandler, error) {
	var handlers []*RouteHandler
	docsContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading handlers %s: %w", path, err)
	}
	if err := json.Unmarshal(docsContent, &handlers); err != nil {
		return nil, fmt.Errorf("parsing handlers %s: %w", path, err)
	}
	return handlers, nil
}

// LoadPublicStructs reads the public_structs.json produced by ExtractStructs.
func LoadPublicStructs(path string) ([]*GoStruct, error) {
	var goStructs []*GoStruct
	structsContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading public structs %s: %w", path, err)
	}
	if err := json.Unmarshal(structsContent, &goStructs); err != nil {
		return nil, fmt.Errorf("parsing public structs %s: %w", path, err)
	}
	return goStructs, nil
}
