package codegen

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// doc splits a doc comment the way GenerateHandlers does, so these tests see
// exactly what the generator sees.
func doc(lines ...string) []string { return lines }

func TestParseHandlerDoc(t *testing.T) {
	t.Run("a fully annotated handler", func(t *testing.T) {
		api := parseHandlerDoc(doc(
			"@summary gets a thing.",
			"@desc First description.",
			"@desc Second description.",
			"@route /api/v1/thing/{id} [GET]",
			`@param id - int - true - "The thing id."`,
			"@returns models.Thing",
		))

		require.Equal(t, "gets a thing.", api.Summary)
		require.Equal(t, []string{"First description.", "Second description."}, api.Descriptions)
		require.Equal(t, "/api/v1/thing/{id}", api.Endpoint)
		require.Equal(t, []string{"GET"}, api.Methods)
		require.Equal(t, "models.Thing", api.Returns)
		require.Equal(t, "models.Thing", api.ReturnGoType)
		require.Equal(t, "Models_Thing", api.ReturnTypescriptType)

		require.Len(t, api.Params, 1)
		require.Equal(t, "id", api.Params[0].Name)
		require.Equal(t, "id", api.Params[0].JsonName)
		require.Equal(t, "int", api.Params[0].GoType)
		require.Equal(t, "number", api.Params[0].TypescriptType)
		require.True(t, api.Params[0].Required)
		require.Equal(t, []string{"The thing id."}, api.Params[0].Descriptions,
			"surrounding quotes are stripped from the description")
	})

	t.Run("multiple methods", func(t *testing.T) {
		api := parseHandlerDoc(doc("@route /api/v1/thing [POST,PATCH,DELETE]"))
		require.Equal(t, []string{"POST", "PATCH", "DELETE"}, api.Methods)
		require.Equal(t, "/api/v1/thing", api.Endpoint)
	})

	t.Run("tag order does not matter", func(t *testing.T) {
		api := parseHandlerDoc(doc(
			"@returns bool",
			"@route /api/v1/thing [GET]",
			"@summary does a thing.",
		))
		require.Equal(t, "does a thing.", api.Summary)
		require.Equal(t, "/api/v1/thing", api.Endpoint)
		require.Equal(t, "bool", api.Returns)
	})

	t.Run("a leading comment marker is tolerated", func(t *testing.T) {
		api := parseHandlerDoc(doc("// @summary gets a thing."))
		require.Equal(t, "gets a thing.", api.Summary)
	})

	t.Run("missing @returns defaults to bool", func(t *testing.T) {
		api := parseHandlerDoc(doc("@route /api/v1/thing [GET]"))
		require.Equal(t, "bool", api.Returns)
		require.Equal(t, "bool", api.ReturnGoType)
		require.Equal(t, "boolean", api.ReturnTypescriptType)
	})

	t.Run("a handler with no tags yields no endpoint and nil methods", func(t *testing.T) {
		api := parseHandlerDoc(doc("Just a regular comment.", "Another line."))

		require.Empty(t, api.Summary)
		require.Empty(t, api.Endpoint)
		// Nil, not empty: it marshals to null in handlers.json, and callers use
		// len(Methods) == 0 to drop the handler from the generated output.
		require.Nil(t, api.Methods)
		require.Empty(t, api.Params)
		require.NotNil(t, api.Params, "an empty slice, so handlers.json has [] not null")
		require.NotNil(t, api.BodyFields)
	})

	t.Run("a malformed @route is ignored", func(t *testing.T) {
		cases := []string{
			"@route /api/v1/thing",             // no methods
			"@route /api/v1/thing [GET] extra", // too many parts
			"@route",                           // nothing at all
		}
		for _, line := range cases {
			t.Run(line, func(t *testing.T) {
				api := parseHandlerDoc(doc(line))
				require.Empty(t, api.Endpoint)
				require.Nil(t, api.Methods)
			})
		}
	})

	t.Run("a malformed @param is ignored", func(t *testing.T) {
		cases := []string{
			"@param id - int - true",                  // 3 parts
			`@param id - int - true - "desc" - extra`, // 5 parts
			"@param", // nothing
		}
		for _, line := range cases {
			t.Run(line, func(t *testing.T) {
				api := parseHandlerDoc(doc(line))
				require.Empty(t, api.Params)
			})
		}
	})

	t.Run("a param is optional when the required column is not exactly true", func(t *testing.T) {
		api := parseHandlerDoc(doc(`@param id - int - false - "The id."`))
		require.Len(t, api.Params, 1)
		require.False(t, api.Params[0].Required)
	})

	t.Run("a struct-typed param has no TypeScript scalar", func(t *testing.T) {
		// goTypeToTypescriptType returns "unknown" for anything non-scalar.
		api := parseHandlerDoc(doc(`@param body - models.User - true - "The user."`))
		require.Equal(t, "unknown", api.Params[0].TypescriptType)
	})

	t.Run("a slice return type is unwrapped for ReturnGoType", func(t *testing.T) {
		api := parseHandlerDoc(doc("@returns []*models.User"))
		require.Equal(t, "[]*models.User", api.Returns)
		require.Equal(t, "models.User", api.ReturnGoType)
		require.Equal(t, "Array<Models_User>", api.ReturnTypescriptType)
	})
}

func TestParseBodyFields(t *testing.T) {
	t.Run("a handler with no body struct has no body fields", func(t *testing.T) {
		fn := parseFuncDecl(t, "func HandleThing(c *RouteCtx) error { return nil }")
		got := parseBodyFields(fn)
		require.NotNil(t, got, "an empty slice, so handlers.json has [] not null")
		require.Empty(t, got)
	})

	t.Run("a local type that is not named body is ignored", func(t *testing.T) {
		fn := parseFuncDecl(t, `
func HandleThing(c *RouteCtx) error {
	type notBody struct {
		Name string `+"`json:\"name\"`"+`
	}
	return nil
}`)
		require.Empty(t, parseBodyFields(fn))
	})

	t.Run("scalars, pointers and omitempty", func(t *testing.T) {
		fn := parseFuncDecl(t, `
func HandleThing(c *RouteCtx) error {
	type body struct {
		Name     string `+"`json:\"name\"`"+`
		Optional int    `+"`json:\"optional,omitempty\"`"+`
		Ptr      *int   `+"`json:\"ptr\"`"+`
		NoTag    bool
	}
	return nil
}`)
		got := parseBodyFields(fn)
		require.Len(t, got, 4)

		require.Equal(t, "name", got[0].JsonName)
		require.True(t, got[0].Required)
		require.Equal(t, "string", got[0].TypescriptType)
		require.Empty(t, got[0].UsedStructType, "a scalar is not a struct reference")

		require.False(t, got[1].Required, "omitempty makes it optional")
		require.False(t, got[2].Required, "a pointer makes it optional")

		require.Equal(t, "NoTag", got[3].JsonName, "with no tag the Go name is used")
		require.True(t, got[3].Required)
	})

	t.Run("a struct-typed field records the struct it references", func(t *testing.T) {
		fn := parseFuncDecl(t, `
func HandleThing(c *RouteCtx) error {
	type body struct {
		Media *anilist.BaseAnime `+"`json:\"media\"`"+`
	}
	return nil
}`)
		got := parseBodyFields(fn)
		require.Len(t, got, 1)
		require.Equal(t, "anilist.BaseAnime", got[0].GoType)
		require.Equal(t, "anilist.BaseAnime", got[0].UsedStructType)
		require.Equal(t, "AL_BaseAnime", got[0].TypescriptType)
		require.False(t, got[0].Required)
	})

	t.Run("inline struct shapes are captured as Go source", func(t *testing.T) {
		fn := parseFuncDecl(t, `
func HandleThing(c *RouteCtx) error {
	type body struct {
		Inline struct {
			Enabled bool `+"`json:\"enabled\"`"+`
		} `+"`json:\"inline\"`"+`
		Slice []struct {
			Value string `+"`json:\"value\"`"+`
		} `+"`json:\"slice\"`"+`
		Mapped map[string]struct {
			Value string `+"`json:\"value\"`"+`
		} `+"`json:\"mapped\"`"+`
		Plain string `+"`json:\"plain\"`"+`
	}
	return nil
}`)
		got := parseBodyFields(fn)
		require.Len(t, got, 4)

		require.Equal(t, "__STRUCT__", got[0].GoType)
		require.Contains(t, got[0].InlineStructType, "struct{")
		require.Contains(t, got[0].InlineStructType, "Enabled bool")

		require.Equal(t, "[]__STRUCT__", got[1].GoType)
		require.True(t, len(got[1].InlineStructType) > 2 && got[1].InlineStructType[:2] == "[]")

		require.Equal(t, "map[string]__STRUCT__", got[2].GoType)
		require.Contains(t, got[2].InlineStructType, "map[string]struct{")

		require.Empty(t, got[3].InlineStructType, "a non-inline field has no inline shape")
	})

	t.Run("only the first body struct is used", func(t *testing.T) {
		fn := parseFuncDecl(t, `
func HandleThing(c *RouteCtx) error {
	type body struct {
		First string `+"`json:\"first\"`"+`
	}
	return nil
}`)
		got := parseBodyFields(fn)
		require.Len(t, got, 1)
		require.Equal(t, "first", got[0].JsonName)
	})
}

// TestParseHandlerDocDuplicatesFieldDescriptions pins a known bug so that fixing
// it is a deliberate act with a visible generated diff.
//
// parseBodyFields appends to fieldComments while ranging over it, so every
// non-empty comment line ends up in the slice twice, with the blank trailing line
// that Doc.Text() produces sitting between them. Committed output shows it:
// HandleSearchTorrent's Type field carries
//
//	["\"smart\" or \"simple\"", "", "\"smart\" or \"simple\""]
//
// and that becomes a duplicated JSDoc block in endpoint.types.ts.
func TestParseBodyFieldsDuplicatesFieldDescriptions(t *testing.T) {
	fn := parseFuncDecl(t, `
func HandleThing(c *RouteCtx) error {
	type body struct {
		// The thing type.
		Type string `+"`json:\"type\"`"+`
	}
	return nil
}`)
	got := parseBodyFields(fn)
	require.Len(t, got, 1)
	require.Equal(t, []string{"The thing type.", "", "The thing type."}, got[0].Descriptions,
		"known bug: the comment is duplicated; see the doc comment on this test")
}

// TestGenerateHandlersOverFixtures exercises the whole file-walking entrypoint.
func TestGenerateHandlersOverFixtures(t *testing.T) {
	outDir := t.TempDir()
	require.NoError(t, GenerateHandlers(filepath.Join("testdata", "handlers"), outDir))

	handlers, err := LoadHandlers(filepath.Join(outDir, "handlers.json"))
	require.NoError(t, err)

	byName := make(map[string]*RouteHandler, len(handlers))
	for _, h := range handlers {
		byName[h.Name] = h
	}

	require.Contains(t, byName, "HandleGetThing")
	require.Contains(t, byName, "HandleSaveThing")
	require.Contains(t, byName, "HandleNoTags")

	get := byName["HandleGetThing"]
	require.Equal(t, "GetThing", get.TrimmedName, "only the Handle prefix is trimmed")
	require.Equal(t, "routes.go", get.Filename)
	require.Equal(t, []string{"GET"}, get.Api.Methods)
	require.Equal(t, "/api/v1/thing/{id}", get.Api.Endpoint)

	save := byName["HandleSaveThing"]
	require.Equal(t, []string{"POST", "PATCH"}, save.Api.Methods)
	require.NotEmpty(t, save.Api.BodyFields)

	require.Nil(t, byName["HandleNoTags"].Api.Methods,
		"a handler with no @route has no methods and is dropped downstream")
}

func TestGenerateHandlersSkipsUnderscorePrefixedFiles(t *testing.T) {
	srcDir := t.TempDir()
	// Files beginning with "_" are ignored by the Go tool and by this generator.
	write := func(name, content string) {
		require.NoError(t, osWriteFile(filepath.Join(srcDir, name), content))
	}
	write("visible.go", "package p\n\n// @route /api/v1/a [GET]\nfunc HandleA(c *RouteCtx) error { return nil }\n")
	write("_hidden.go", "package p\n\n// @route /api/v1/b [GET]\nfunc HandleB(c *RouteCtx) error { return nil }\n")

	outDir := t.TempDir()
	require.NoError(t, GenerateHandlers(srcDir, outDir))

	handlers, err := LoadHandlers(filepath.Join(outDir, "handlers.json"))
	require.NoError(t, err)
	require.Len(t, handlers, 1)
	require.Equal(t, "HandleA", handlers[0].Name)
}

func TestGenerateHandlersReturnsErrorOnUnparseableSource(t *testing.T) {
	srcDir := t.TempDir()
	require.NoError(t, osWriteFile(filepath.Join(srcDir, "broken.go"), "package p\nfunc ("))

	err := GenerateHandlers(srcDir, t.TempDir())
	require.Error(t, err, "a syntax error must fail the run rather than silently skip the file")
}

// sanity check that the fixture handlers themselves parse, so a broken fixture
// fails loudly here rather than as a confusing assertion elsewhere.
func TestHandlerFixturesParse(t *testing.T) {
	_, err := parser.ParseFile(token.NewFileSet(),
		filepath.Join("testdata", "handlers", "routes.go"), nil, parser.ParseComments)
	require.NoError(t, err)
}
