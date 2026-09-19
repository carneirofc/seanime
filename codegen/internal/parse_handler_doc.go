package codegen

import (
	"go/ast"
	"go/token"
	"strings"
)

// This file owns the authoring contract for route handlers: the doc-comment tags
// a handler declares, and the local `body` struct it uses to describe its request
// body. Both are parsed here and nowhere else.
//
// A handler looks like this:
//
//	// @summary returns the anime collection.
//	// @desc This is cached.
//	// @route /api/v1/anilist/collection [GET]
//	// @param id - int - true - "The media id."
//	// @returns anilist.AnimeCollection
//	func HandleGetAnimeCollection(c *RouteCtx) error {
//		type body struct {
//			MediaId int `json:"mediaId"`
//		}
//	}

// parseHandlerDoc turns a handler's doc comment into a RouteHandlerApi.
//
// Tags are recognized anywhere in the comment and in any order. Unrecognized
// lines are ignored. A malformed tag is skipped rather than failing the run —
// the generator has no way to report a diagnostic back to the author, so a
// handler with a broken @route simply produces no endpoint and is dropped from
// the generated output by the callers that require Methods to be non-empty.
func parseHandlerDoc(comments []string) *RouteHandlerApi {
	endpoint := ""
	// Deliberately nil rather than an empty slice: it marshals to `null` in
	// handlers.json for a handler with no @route, which is what consumers
	// already expect.
	var methods []string
	params := make([]*RouteHandlerParam, 0)
	summary := ""
	descriptions := make([]string, 0)
	returns := "bool"

	for _, comment := range comments {
		cmt := strings.TrimSpace(strings.TrimPrefix(comment, "//"))

		if strings.HasPrefix(cmt, "@summary") {
			summary = strings.TrimSpace(strings.TrimPrefix(cmt, "@summary"))
		}

		if strings.HasPrefix(cmt, "@desc") {
			descriptions = append(descriptions, strings.TrimSpace(strings.TrimPrefix(cmt, "@desc")))
		}

		// @route /api/v1/path [GET,POST]
		if strings.HasPrefix(cmt, "@route") {
			endpointParts := strings.Split(strings.TrimSpace(strings.TrimPrefix(cmt, "@route")), " ")
			if len(endpointParts) == 2 {
				endpoint = endpointParts[0]
				methods = strings.Split(endpointParts[1][1:len(endpointParts[1])-1], ",")
			}
		}

		// @param name - goType - required - "description"
		if strings.HasPrefix(cmt, "@param") {
			paramParts := strings.Split(strings.TrimSpace(strings.TrimPrefix(cmt, "@param")), " - ")
			if len(paramParts) == 4 {
				required := paramParts[2] == "true"
				params = append(params, &RouteHandlerParam{
					Name:           paramParts[0],
					JsonName:       paramParts[0],
					GoType:         paramParts[1],
					TypescriptType: goTypeToTypescriptType(paramParts[1]),
					Required:       required,
					Descriptions:   []string{strings.ReplaceAll(paramParts[3], "\"", "")},
				})
			}
		}

		if strings.HasPrefix(cmt, "@returns") {
			returns = strings.TrimSpace(strings.TrimPrefix(cmt, "@returns"))
		}
	}

	return &RouteHandlerApi{
		Summary:              summary,
		Descriptions:         descriptions,
		Endpoint:             endpoint,
		Methods:              methods,
		Params:               params,
		BodyFields:           make([]*RouteHandlerParam, 0),
		Returns:              returns,
		ReturnGoType:         getUnformattedGoType(returns),
		ReturnTypescriptType: stringGoTypeToTypescriptType(returns),
	}
}

// parseBodyFields extracts the request body shape from a handler function body.
//
// The convention is a local type declaration named exactly `body`; a handler
// without one has no body fields. Only the first such declaration is used.
func parseBodyFields(fn *ast.FuncDecl) []*RouteHandlerParam {
	bodyFields := make([]*RouteHandlerParam, 0)

	if fn.Body == nil {
		return bodyFields
	}

	for _, stmt := range fn.Body.List {
		structType, ok := localBodyStruct(stmt)
		if !ok {
			continue
		}

		for _, field := range structType.Fields.List {
			// Get the field name
			fieldName := field.Names[0].Name

			// Get the field type
			fieldType := field.Type

			jsonName := fieldName
			// Get the field tag
			required := !jsonFieldOmitEmpty(field)
			jsonField := jsonFieldName(field)
			if jsonField != "" {
				jsonName = jsonField
			}

			// Get field comments.
			//
			// Doc.Text() returns the comment with a trailing newline, so a split
			// yields a final empty element that must not be emitted.
			fieldComments := make([]string, 0)
			for _, line := range strings.Split(field.Doc.Text(), "\n") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "//"))
				if line != "" {
					fieldComments = append(fieldComments, line)
				}
			}

			switch fieldType.(type) {
			case *ast.StarExpr:
				required = false
			}

			goType := fieldTypeString(fieldType)
			goTypeUnformatted := fieldTypeUnformattedString(fieldType)
			packageName := "handlers"
			if strings.Contains(goTypeUnformatted, ".") {
				parts := strings.Split(goTypeUnformatted, ".")
				packageName = parts[0]
			}

			tsType := fieldTypeToTypescriptType(fieldType, packageName)

			usedStructType := goTypeUnformatted
			switch goTypeUnformatted {
			case "string", "int", "int64", "float64", "float32", "bool", "nil", "uint", "uint64", "uint32", "uint16", "uint8", "byte", "rune", "[]byte", "interface{}", "error":
				usedStructType = ""
			}

			// Add the request body field
			bodyFields = append(bodyFields, &RouteHandlerParam{
				Name:           fieldName,
				JsonName:       jsonName,
				GoType:         goType,
				UsedStructType: usedStructType,
				TypescriptType: tsType,
				Required:       required,
				Descriptions:   fieldComments,
			})

			bodyFields[len(bodyFields)-1].InlineStructType = inlineStructType(fieldType)
		}
	}

	return bodyFields
}

// localBodyStruct reports whether stmt is a `type body struct { ... }`
// declaration and returns its struct type.
func localBodyStruct(stmt ast.Stmt) (*ast.StructType, bool) {
	declStmt, ok := stmt.(*ast.DeclStmt)
	if !ok {
		return nil, false
	}
	genDecl, ok := declStmt.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.TYPE || len(genDecl.Specs) != 1 {
		return nil, false
	}
	typeSpec, ok := genDecl.Specs[0].(*ast.TypeSpec)
	if !ok || typeSpec.Name.Name != "body" {
		return nil, false
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return nil, false
	}
	return structType, true
}

// inlineStructType renders an anonymous struct type (or a slice/map of one) as
// Go source, so the generated hook events can restate it. It returns "" for any
// type that is not built on an anonymous struct.
func inlineStructType(fieldType ast.Expr) string {
	switch t := fieldType.(type) {
	case *ast.StructType:
		return formatInlineStruct(t)
	case *ast.ArrayType:
		if structType, ok := t.Elt.(*ast.StructType); ok {
			return "[]" + formatInlineStruct(structType)
		}
	case *ast.MapType:
		if structType, ok := t.Value.(*ast.StructType); ok {
			return "map[" + fieldTypeString(t.Key) + "]" + formatInlineStruct(structType)
		}
	}
	return ""
}
