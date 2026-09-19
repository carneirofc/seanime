package codegen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// These tests render emitters into a buffer and compare the result against
// testdata/golden. They are the safety net for changing how output LOOKS;
// refresh them with -update and read the diff.

func TestEmitTypescriptType_Struct(t *testing.T) {
	goStruct := &GoStruct{
		Filepath:      "../internal/database/models/user.go",
		Filename:      "user.go",
		Name:          "User",
		FormattedName: "Models_User",
		Package:       "models",
		Comments:      []string{" User is a person.", " Second line."},
		Fields: []*GoStructField{
			{Name: "ID", JsonName: "id", TypescriptType: "number", Required: true},
			{Name: "Name", JsonName: "name", TypescriptType: "string", Required: true,
				Comments: []string{" The display name."}},
			{Name: "Nickname", JsonName: "nickname", TypescriptType: "string", Required: false},
			{Name: "Meta", JsonName: "meta", TypescriptType: "RawMessage", Required: false},
			// A field with no JSON name is skipped entirely.
			{Name: "Secret", JsonName: "", TypescriptType: "string", Required: true},
		},
	}

	got := render(t, func(w *errWriter) {
		writeTypescriptType(w, goStruct, map[string]*GoStruct{})
	})
	assertGolden(t, "type_struct.ts", got)
}

func TestEmitTypescriptType_RecordsWrittenType(t *testing.T) {
	written := map[string]*GoStruct{}
	goStruct := &GoStruct{
		Name: "User", FormattedName: "Models_User", Package: "models",
		Fields: []*GoStructField{{Name: "ID", JsonName: "id", TypescriptType: "number", Required: true}},
	}

	render(t, func(w *errWriter) { writeTypescriptType(w, goStruct, written) })

	require.Contains(t, written, "models.User",
		"emitting a type must record it so later collisions can be detected")
}

func TestEmitTypescriptType_ShortUnionIsSingleLine(t *testing.T) {
	goStruct := &GoStruct{
		Filepath: "../internal/api/anilist/status.go", Filename: "status.go",
		Name: "MediaListStatus", FormattedName: "AL_MediaListStatus", Package: "anilist",
		Fields: []*GoStructField{},
		AliasOf: &GoAlias{
			GoType:         "string",
			TypescriptType: "string",
			DeclaredValues: []string{`"CURRENT"`, `"PLANNING"`, `"COMPLETED"`},
		},
	}

	got := render(t, func(w *errWriter) {
		writeTypescriptType(w, goStruct, map[string]*GoStruct{})
	})
	assertGolden(t, "type_union_short.ts", got)
}

func TestEmitTypescriptType_LongUnionWrapsOntoMultipleLines(t *testing.T) {
	// More than five declared values switches to one per line.
	goStruct := &GoStruct{
		Filepath: "../internal/api/anilist/sort.go", Filename: "sort.go",
		Name: "MediaSort", FormattedName: "AL_MediaSort", Package: "anilist",
		Fields: []*GoStructField{},
		AliasOf: &GoAlias{
			GoType:         "string",
			TypescriptType: "string",
			DeclaredValues: []string{`"ID"`, `"ID_DESC"`, `"TITLE"`, `"TITLE_DESC"`, `"SCORE"`, `"SCORE_DESC"`},
		},
	}

	got := render(t, func(w *errWriter) {
		writeTypescriptType(w, goStruct, map[string]*GoStruct{})
	})
	assertGolden(t, "type_union_long.ts", got)
}

func TestEmitTypescriptType_AliasWithoutDeclaredValues(t *testing.T) {
	goStruct := &GoStruct{
		Filepath: "../internal/p/list.go", Filename: "list.go",
		Name: "List", FormattedName: "P_List", Package: "p",
		Fields:  []*GoStructField{},
		AliasOf: &GoAlias{GoType: "[]User", TypescriptType: "Array<P_User>"},
	}

	got := render(t, func(w *errWriter) {
		writeTypescriptType(w, goStruct, map[string]*GoStruct{})
	})
	assertGolden(t, "type_alias_plain.ts", got)
}

func TestEmitParamField(t *testing.T) {
	handler := &RouteHandler{Name: "HandleSaveThing"}

	got := render(t, func(w *errWriter) {
		writeParamField(w, handler, &RouteHandlerParam{
			JsonName: "required", TypescriptType: "string", Required: true,
		})
		writeParamField(w, handler, &RouteHandlerParam{
			JsonName: "optional", TypescriptType: "number", Required: false,
		})
		writeParamField(w, handler, &RouteHandlerParam{
			JsonName: "documented", TypescriptType: "boolean", Required: true,
			Descriptions: []string{"First line.", "", "Third line."},
		})
		writeParamField(w, handler, &RouteHandlerParam{
			JsonName: "raw", TypescriptType: "RawMessage", Required: true,
		})
	})
	assertGolden(t, "param_fields.ts", got)
}

func TestEmitHooksFile(t *testing.T) {
	handlers := map[string][]*RouteHandler{
		"anilist.go": {
			{
				Name: "HandleGetAnimeCollection",
				Api: &RouteHandlerApi{
					Endpoint:             "/api/v1/anilist/collection",
					Methods:              []string{"GET"},
					ReturnTypescriptType: "AL_AnimeCollection",
					Params:               []*RouteHandlerParam{},
					BodyFields:           []*RouteHandlerParam{},
				},
			},
			{
				Name: "HandleGetThingById",
				Api: &RouteHandlerApi{
					Endpoint:             "/api/v1/anilist/thing/{id}",
					Methods:              []string{"GET"},
					ReturnTypescriptType: "AL_Thing",
					Params:               []*RouteHandlerParam{{JsonName: "id", TypescriptType: "number"}},
					BodyFields:           []*RouteHandlerParam{},
				},
			},
			{
				Name: "HandleSaveThing",
				Api: &RouteHandlerApi{
					Endpoint:             "/api/v1/anilist/thing",
					Methods:              []string{"POST"},
					ReturnTypescriptType: "AL_Thing",
					Params:               []*RouteHandlerParam{},
					BodyFields:           []*RouteHandlerParam{{JsonName: "name", TypescriptType: "string"}},
				},
			},
			{
				// Two methods produce two hooks, each indexing methods[i].
				Name: "HandleMultiMethod",
				Api: &RouteHandlerApi{
					Endpoint:             "/api/v1/anilist/multi",
					Methods:              []string{"GET", "DELETE"},
					ReturnTypescriptType: "",
					Params:               []*RouteHandlerParam{},
					BodyFields:           []*RouteHandlerParam{},
				},
			},
		},
	}

	got := render(t, func(w *errWriter) {
		generateHooksFile(w, handlers, []string{"anilist.go"})
	})
	assertGolden(t, "hooks_template.ts", got)
}

func TestEmitHooksFile_SkipsFilesWithNoRoutedHandlers(t *testing.T) {
	handlers := map[string][]*RouteHandler{
		"helpers.go": {
			{Name: "HandleNotARoute", Api: &RouteHandlerApi{Methods: nil}},
		},
	}

	got := render(t, func(w *errWriter) {
		generateHooksFile(w, handlers, []string{"helpers.go"})
	})
	require.Empty(t, got, "a file whose handlers have no methods produces no output at all")
}

func TestWriteLineExpandsTabsToTheConfiguredIndent(t *testing.T) {
	got := render(t, func(w *errWriter) {
		writeLine(w, "\tone")
		writeLine(w, "\t\ttwo")
		writeLine(w, "none")
	})
	require.Equal(t, space+"one\n"+space+space+"two\n"+"none\n", got)
}

func TestErrWriterStopsAfterFirstError(t *testing.T) {
	w := newErrWriter(failingWriter{})
	w.WriteString("first")
	w.WriteString("second")
	require.Error(t, w.Err())
}
