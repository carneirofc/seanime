package codegen

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type (
	RouteHandler struct {
		Name        string           `json:"name"`
		TrimmedName string           `json:"trimmedName"`
		Comments    []string         `json:"comments"`
		Filepath    string           `json:"filepath"`
		Filename    string           `json:"filename"`
		Api         *RouteHandlerApi `json:"api"`
	}

	RouteHandlerApi struct {
		Summary              string               `json:"summary"`
		Descriptions         []string             `json:"descriptions"`
		Endpoint             string               `json:"endpoint"`
		Methods              []string             `json:"methods"`
		Params               []*RouteHandlerParam `json:"params"`
		BodyFields           []*RouteHandlerParam `json:"bodyFields"`
		Returns              string               `json:"returns"`
		ReturnGoType         string               `json:"returnGoType"`
		ReturnTypescriptType string               `json:"returnTypescriptType"`
	}

	RouteHandlerParam struct {
		Name             string   `json:"name"`
		JsonName         string   `json:"jsonName"`
		GoType           string   `json:"goType"`                     // e.g., []models.User
		InlineStructType string   `json:"inlineStructType,omitempty"` // e.g., struct{Test string `json:"test"`}
		UsedStructType   string   `json:"usedStructType"`             // e.g., models.User
		TypescriptType   string   `json:"typescriptType"`             // e.g., Array<User>
		Required         bool     `json:"required"`
		Descriptions     []string `json:"descriptions"`
	}
)

// GenerateHandlers walks dir for route handler functions and writes their parsed
// API contract to outDir/handlers.json.
func GenerateHandlers(dir string, outDir string) error {

	handlers := make([]*RouteHandler, 0)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasPrefix(info.Name(), "_") {
			return nil
		}

		// Parse the file
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, decl := range file.Decls {
			// Check if the declaration is a function
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Check if the function has comments
			if fn.Doc == nil {
				continue
			}

			// Get the comments
			comments := strings.Split(fn.Doc.Text(), "\n")
			if len(comments) == 0 {
				continue
			}

			// Get the function name
			name := fn.Name.Name
			trimmedName := strings.TrimPrefix(name, "Handle")

			// Get the filename
			filep := strings.ReplaceAll(strings.ReplaceAll(path, "\\", "/"), "../", "")
			filename := filepath.Base(path)

			api := parseHandlerDoc(comments)
			api.BodyFields = parseBodyFields(fn)

			// Add the route handler
			routeHandler := &RouteHandler{
				Name:        name,
				TrimmedName: trimmedName,
				Comments:    comments,
				Filepath:    filep,
				Filename:    filename,
				Api:         api,
			}

			handlers = append(handlers, routeHandler)

		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walking %s: %w", dir, err)
	}

	// Write handlers to file
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return fmt.Errorf("creating %s: %w", outDir, err)
	}
	outPath := filepath.Join(outDir, "handlers.json")
	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(handlers); err != nil {
		return fmt.Errorf("encoding %s: %w", outPath, err)
	}

	return nil
}
