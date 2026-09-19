package codegen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTsTypeToZod(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Primitives.
		{"string", "string", "z.string()"},
		{"number", "number", "z.number()"},
		{"boolean", "boolean", "z.boolean()"},
		{"null", "null", "z.null()"},
		{"any becomes unknown", "any", "z.unknown()"},

		// Go json.RawMessage.
		{"raw message", "Record<string, any>", "z.record(z.string(), z.unknown())"},

		// Arrays.
		{"array of primitives", "Array<string>", "z.array(z.string())"},
		{"array of generated type", "Array<Models_User>", "z.array(Models_UserSchema)"},
		{"nested arrays", "Array<Array<number>>", "z.array(z.array(z.number()))"},

		// Records. A Go map[int]T is a JSON object with STRING keys, so a numeric
		// key type must coerce or every real payload is rejected.
		{"record of string keys", "Record<string, string>", "z.record(z.string(), z.string())"},
		{"record of generated type", "Record<string, Models_User>", "z.record(z.string(), Models_UserSchema)"},
		{"record of numeric keys coerces", "Record<number, string>", "z.record(z.coerce.number(), z.string())"},
		{
			name: "record of numeric keys to generated type",
			in:   "Record<number, Continuity_WatchHistoryItem>",
			want: "z.record(z.coerce.number(), Continuity_WatchHistoryItemSchema)",
		},

		// Nesting that exercises the top-level split: the comma inside Array<>
		// must not be read as the record's key/value separator.
		{
			name: "record of array",
			in:   "Record<string, Array<Manga_ProviderDownloadMapChapterInfo>>",
			want: "z.record(z.string(), z.array(Manga_ProviderDownloadMapChapterInfoSchema))",
		},
		{
			name: "record of array of numbers",
			in:   "Record<number, Array<string>>",
			want: "z.record(z.coerce.number(), z.array(z.string()))",
		},

		// References to other generated types.
		{"generated type", "Models_User", "Models_UserSchema"},
		{"unprefixed generated type", "Video", "VideoSchema"},

		// Anonymous object literals.
		{
			name: "inline object",
			in:   "{ image_url: string; small_image_url: string; }",
			want: `z.looseObject({ image_url: z.string(), small_image_url: z.string() })`,
		},
		{
			name: "inline object with one field",
			in:   "{ image_url: string; }",
			want: `z.looseObject({ image_url: z.string() })`,
		},
		{
			name: "inline object with a nested array",
			in:   "{ items: Array<Models_User>; count: number; }",
			want: `z.looseObject({ items: z.array(Models_UserSchema), count: z.number() })`,
		},
		{
			name: "inline object with an optional field",
			in:   "{ name?: string; }",
			want: `z.looseObject({ name: z.string().nullish() })`,
		},
		{"empty inline object", "{}", "z.looseObject({})"},

		// Fallbacks. An unrecognized expression is not validated rather than
		// failing the generator.
		{"empty string", "", "z.unknown()"},
		{"lowercase unknown identifier", "someLocalThing", "z.unknown()"},
		{"function type", "() => void", "z.unknown()"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tsTypeToZod(tc.in))
		})
	}
}

func TestFieldSchemaAppliesNullishExactlyWhenOptional(t *testing.T) {
	t.Run("a required field is not nullish", func(t *testing.T) {
		got := fieldSchema(&GoStructField{TypescriptType: "string", Required: true})
		require.Equal(t, "z.string()", got)
	})

	t.Run("an optional field accepts null and undefined", func(t *testing.T) {
		// This is the single most important behavior in the file: Go marshals a
		// nil pointer/slice/map as null, not as an absent key.
		got := fieldSchema(&GoStructField{TypescriptType: "string", Required: false})
		require.Equal(t, "z.string().nullish()", got)
	})

	t.Run("nullish wraps the whole composite, not its element", func(t *testing.T) {
		got := fieldSchema(&GoStructField{TypescriptType: "Array<Models_User>", Required: false})
		require.Equal(t, "z.array(Models_UserSchema).nullish()", got)
	})
}

func TestParamSchemaAppliesNullishExactlyWhenOptional(t *testing.T) {
	require.Equal(t, "z.number()",
		paramSchema(&RouteHandlerParam{TypescriptType: "number", Required: true}, ""))
	require.Equal(t, "z.number().nullish()",
		paramSchema(&RouteHandlerParam{TypescriptType: "number", Required: false}, ""))
}

func TestSplitTopLevel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		sep  rune
		want []string
	}{
		{"simple", "a, b", ',', []string{"a", "b"}},
		{"nested angle brackets are not split", "string, Array<a, b>", ',', []string{"string", "Array<a, b>"}},
		{"nested braces are not split", "a, { x: 1; y: 2 }", ',', []string{"a", "{ x: 1; y: 2 }"}},
		{"semicolons", "a: string; b: number", ';', []string{"a: string", "b: number"}},
		{"trailing separator produces no empty entry", "a: string;", ';', []string{"a: string"}},
		{"single element", "a", ',', []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, splitTopLevel(tc.in, tc.sep))
		})
	}
}

func TestQuoteJSKey(t *testing.T) {
	cases := []struct{ in, want string }{
		// Bare identifiers are left unquoted for readability.
		{"name", "name"},
		{"image_url", "image_url"},
		{"$ref", "$ref"},
		{"a1", "a1"},
		// A json tag can carry characters a bare JS key cannot.
		{"content-type", `"content-type"`},
		{"1st", `"1st"`},
		{"with space", `"with space"`},
		{"", `""`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, quoteJSKey(tc.in))
		})
	}
}

func TestIsGeneratedTypeName(t *testing.T) {
	for _, in := range []string{"Models_User", "Video", "AL_BaseAnime", "A1"} {
		require.True(t, isGeneratedTypeName(in), in)
	}
	for _, in := range []string{"string", "any", "", "Array<string>", "{ a: string }", "lower"} {
		require.False(t, isGeneratedTypeName(in), in)
	}
}
