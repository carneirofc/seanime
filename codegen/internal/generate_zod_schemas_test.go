package codegen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// zodFixture builds a selectedStructs from hand-written structs, so ordering and
// cycle handling can be tested without the full pipeline.
func zodFixture(structs ...*GoStruct) *selectedStructs {
	sel := &selectedStructs{
		ByKey:     make(map[string]*GoStruct, len(structs)),
		ByPackage: make(map[string][]*GoStruct),
	}
	for _, s := range structs {
		sel.ByKey[structKey(s)] = s
		sel.ByPackage[s.Package] = append(sel.ByPackage[s.Package], s)
	}
	for pkg := range sel.ByPackage {
		sel.Packages = append(sel.Packages, pkg)
	}
	return sel
}

func objStruct(pkg, name string, fields ...*GoStructField) *GoStruct {
	return &GoStruct{
		Package: pkg, Name: name, FormattedName: getTypePrefix(pkg) + name,
		Filename: name + ".go", Fields: fields,
	}
}

func refField(jsonName, tsType, usedStructType string, required bool) *GoStructField {
	return &GoStructField{
		Name: jsonName, JsonName: jsonName, TypescriptType: tsType,
		UsedStructType: usedStructType, Required: required,
	}
}

func renderSchemas(t *testing.T, sel *selectedStructs) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, writeZodSchemasFile(sel, dir))

	got, err := filepath.Glob(filepath.Join(dir, zodSchemasFileName))
	require.NoError(t, err)
	require.Len(t, got, 1)

	return readFileString(t, got[0])
}

func TestWriteZodSchemas_ObjectAndScalars(t *testing.T) {
	sel := zodFixture(objStruct("models", "User",
		refField("id", "number", "", true),
		refField("name", "string", "", true),
		refField("nickname", "string", "", false),
		// json:"-" - never serialized, so it must not appear in the schema.
		&GoStructField{Name: "Secret", JsonName: "", TypescriptType: "string", Required: true},
	))

	assertGolden(t, "zod_object.ts", renderSchemas(t, sel))
}

func TestWriteZodSchemas_Enum(t *testing.T) {
	sel := zodFixture(&GoStruct{
		Package: "anilist", Name: "MediaListStatus", FormattedName: "AL_MediaListStatus",
		Filename: "status.go", Fields: []*GoStructField{},
		AliasOf: &GoAlias{
			GoType: "string", TypescriptType: "string",
			DeclaredValues: []string{`"CURRENT"`, `"PLANNING"`},
		},
	})

	got := renderSchemas(t, sel)
	assertGolden(t, "zod_enum.ts", got)

	// The values array exists so the UI can build option lists from it.
	require.Contains(t, got, `export const AL_MediaListStatusValues = ["CURRENT", "PLANNING"] as const`)
	require.Contains(t, got, `export const AL_MediaListStatusSchema = z.enum(AL_MediaListStatusValues)`)
}

func TestWriteZodSchemas_PlainAlias(t *testing.T) {
	sel := zodFixture(&GoStruct{
		Package: "models", Name: "IdList", FormattedName: "Models_IdList",
		Filename: "ids.go", Fields: []*GoStructField{},
		AliasOf: &GoAlias{GoType: "[]int", TypescriptType: "Array<number>"},
	})

	require.Contains(t, renderSchemas(t, sel), "export const Models_IdListSchema = z.array(z.number())")
}

func TestWriteZodSchemas_DeclarationOrderFollowsDependencies(t *testing.T) {
	// B references A, so A must be declared first regardless of alphabetical order.
	a := objStruct("p", "Aaa", refField("n", "number", "", true))
	b := objStruct("p", "Bbb", refField("a", "Aaa", "p.Aaa", true))
	// Deliberately register B first.
	sel := zodFixture(b, a)

	got := renderSchemas(t, sel)
	require.Less(t, strings.Index(got, "export const AaaSchema"), strings.Index(got, "export const BbbSchema"),
		"a schema must not be referenced before it is declared")
}

func TestWriteZodSchemas_SelfReferenceBecomesGetter(t *testing.T) {
	// The real tree has exactly one of these: LibraryExplorer_FileTreeNodeJSON.
	node := objStruct("tree", "Node",
		refField("name", "string", "", true),
		refField("children", "Array<Node>", "tree.Node", false),
	)
	sel := zodFixture(node)

	got := renderSchemas(t, sel)
	require.Contains(t, got, "get children() { return z.array(NodeSchema).nullish() }",
		"a self-reference must be deferred to a getter or the const is used before initialization")
	assertGolden(t, "zod_recursive.ts", got)
}

func TestWriteZodSchemas_MutualCycleBecomesGetter(t *testing.T) {
	// No mutual cycle exists in the real tree, so the general case needs a fixture:
	// A -> B -> A.
	a := objStruct("p", "Aaa", refField("b", "Bbb", "p.Bbb", false))
	b := objStruct("p", "Bbb", refField("a", "Aaa", "p.Aaa", false))
	sel := zodFixture(a, b)

	got := renderSchemas(t, sel)
	require.Contains(t, got, "get ", "the edge closing the cycle must become a getter")

	// Whichever schema is emitted second must reference the first through a getter,
	// and the file must still declare both exactly once.
	require.Equal(t, 1, strings.Count(got, "export const AaaSchema"))
	require.Equal(t, 1, strings.Count(got, "export const BbbSchema"))
}

func TestZodEmissionOrderRejectsDuplicateNames(t *testing.T) {
	// Two packages sharing a prefix can format to the same TypeScript name, which
	// would declare the same const twice.
	sel := zodFixture(
		&GoStruct{Package: "habari", Name: "Metadata", FormattedName: "Habari_Metadata"},
		&GoStruct{Package: "vendor_habari", Name: "Metadata", FormattedName: "Habari_Metadata"},
	)

	_, _, err := zodEmissionOrder(sel)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate generated type name")
}

func TestReferencedSchemaNames(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"z.string()", nil},
		{"Models_UserSchema", []string{"Models_UserSchema"}},
		{"z.array(Models_UserSchema).nullish()", []string{"Models_UserSchema"}},
		{"z.record(z.string(), Models_UserSchema)", []string{"Models_UserSchema"}},
		{"s.Models_UserSchema", []string{"Models_UserSchema"}},
		{
			in:   "z.looseObject({ a: Models_UserSchema, b: Models_ThemeSchema })",
			want: []string{"Models_UserSchema", "Models_ThemeSchema"},
		},
		// "Schema" alone is the suffix, not a reference.
		{"z.unknown()", nil},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, referencedSchemaNames(tc.in))
		})
	}
}

func TestEndpointPattern(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/api/v1/thing", `/^\/api\/v1\/thing$/`},
		// A placeholder matches one segment only, so /a/{id} cannot swallow /a/b/c.
		{"/api/v1/thing/{id}", `/^\/api\/v1\/thing\/[^\/]+$/`},
		{"/api/v1/a/{id}/b/{name}", `/^\/api\/v1\/a\/[^\/]+\/b\/[^\/]+$/`},
		// Regex metacharacters in a literal segment are escaped.
		{"/api/v1/a.b", `/^\/api\/v1\/a\.b$/`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, endpointPattern(tc.in))
		})
	}
}

func TestResolvableDegradesUnknownSchemas(t *testing.T) {
	generated := map[string]bool{"Models_UserSchema": true}

	require.Equal(t, "z.array(Models_UserSchema)",
		resolvable("z.array(Models_UserSchema)", generated))
	require.Equal(t, "z.string()", resolvable("z.string()", generated))
	// A handler can return a type no struct in the selected set reaches; referencing
	// its schema would not compile.
	require.Equal(t, zodUnknown,
		resolvable("TorrentDetailsSchema", generated))
}

func TestWriteZodEndpointSchemas_OmitsAmbiguousEndpoints(t *testing.T) {
	// Two handlers declaring the same route and method: which response schema applies
	// cannot be determined. The real tree has exactly one such pair.
	handlers := []*RouteHandler{
		{
			Name: "HandleReloadA", Filename: "extensions.go",
			Api: &RouteHandlerApi{
				Endpoint: "/api/v1/dup", Methods: []string{"POST"},
				ReturnTypescriptType: "boolean",
				Params:               []*RouteHandlerParam{}, BodyFields: []*RouteHandlerParam{},
			},
		},
		{
			Name: "HandleReloadB", Filename: "extensions.go",
			Api: &RouteHandlerApi{
				Endpoint: "/api/v1/dup", Methods: []string{"POST"},
				ReturnTypescriptType: "boolean",
				Params:               []*RouteHandlerParam{}, BodyFields: []*RouteHandlerParam{},
			},
		},
		{
			Name: "HandleFine", Filename: "extensions.go",
			Api: &RouteHandlerApi{
				Endpoint: "/api/v1/fine", Methods: []string{"GET"},
				ReturnTypescriptType: "string",
				Params:               []*RouteHandlerParam{}, BodyFields: []*RouteHandlerParam{},
			},
		},
	}

	dir := t.TempDir()
	require.NoError(t, writeZodEndpointSchemasFile(handlers, map[string]bool{}, dir))
	got := readFileString(t, filepath.Join(dir, zodEndpointSchemasFileName))

	require.NotContains(t, got, `"POST /api/v1/dup"`, "an ambiguous endpoint must be omitted, not guessed at")
	require.Contains(t, got, "Omitted as ambiguous", "and it must say why")
	require.Contains(t, got, `"GET /api/v1/fine"`, "unambiguous endpoints are unaffected")
}

func TestWriteZodEndpointSchemas_VariablesAndLookups(t *testing.T) {
	handlers := []*RouteHandler{
		{
			Name: "HandleGetThing", Filename: "things.go",
			Api: &RouteHandlerApi{
				Endpoint: "/api/v1/thing/{id}", Methods: []string{"GET"},
				ReturnTypescriptType: "Models_Thing",
				Params:               []*RouteHandlerParam{{Name: "id", JsonName: "id", TypescriptType: "number", Required: true}},
				BodyFields:           []*RouteHandlerParam{},
			},
		},
		{
			Name: "HandleSaveThing", Filename: "things.go",
			Api: &RouteHandlerApi{
				Endpoint: "/api/v1/thing", Methods: []string{"POST"},
				ReturnTypescriptType: "boolean",
				Params:               []*RouteHandlerParam{},
				BodyFields: []*RouteHandlerParam{
					{Name: "Name", JsonName: "name", TypescriptType: "string", Required: true},
					{Name: "Count", JsonName: "count", TypescriptType: "number", Required: false},
				},
			},
		},
	}

	dir := t.TempDir()
	require.NoError(t, writeZodEndpointSchemasFile(handlers, map[string]bool{"Models_ThingSchema": true}, dir))
	assertGolden(t, "zod_endpoint_schemas.ts", readFileString(t, filepath.Join(dir, zodEndpointSchemasFileName)))
}

func TestWriteZodAssertFile(t *testing.T) {
	sel := zodFixture(
		objStruct("models", "User", refField("id", "number", "", true)),
		// An enum has no field set, so there is nothing to assert about it.
		&GoStruct{
			Package: "anilist", Name: "Status", FormattedName: "AL_Status", Fields: []*GoStructField{},
			AliasOf: &GoAlias{TypescriptType: "string", DeclaredValues: []string{`"A"`}},
		},
		// Every field is json:"-", so types.ts emits no body for it.
		objStruct("models", "Hidden", &GoStructField{Name: "X", JsonName: "", TypescriptType: "string"}),
	)

	dir := t.TempDir()
	require.NoError(t, writeZodAssertFile(sel, dir))
	got := readFileString(t, filepath.Join(dir, zodAssertFileName))

	// The comparison is on .shape, not z.infer: a loose object's inferred type carries
	// an index signature that widens keyof to string, making the assertion vacuous.
	require.Contains(t, got, "keyof typeof s.Models_UserSchema.shape")
	require.Contains(t, got, "keyof T.Models_User")
	require.NotContains(t, got, "AL_Status", "an enum has no field set to compare")
	require.NotContains(t, got, "Models_Hidden", "a struct with no serialized field has no type body")
}
