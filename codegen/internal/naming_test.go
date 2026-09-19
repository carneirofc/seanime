package codegen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetEndpointKey pins the shape of the keys in endpoints.ts, e.g.
// "ANILIST-get-anime-collection". Note the group name keeps the casing the caller
// passes (callers upper-case the filename); only the handler part is lowercased.
func TestGetEndpointKey(t *testing.T) {
	cases := []struct {
		name      string
		handler   string
		groupName string
		want      string
	}{
		{
			name:      "camel case becomes kebab case",
			handler:   "HandleGetAnimeCollection",
			groupName: "ANILIST",
			want:      "ANILIST-get-anime-collection",
		},
		{
			name:      "the Handle prefix is dropped",
			handler:   "HandleX",
			groupName: "G",
			want:      "G-x",
		},
		{
			name:      "a name without the Handle prefix is still accepted",
			handler:   "GetThing",
			groupName: "G",
			want:      "G-get-thing",
		},
		{
			name:      "underscores in the group name become dashes",
			handler:   "HandleGetThing",
			groupName: "TORRENT_CLIENT",
			want:      "TORRENT-CLIENT-get-thing",
		},
		{
			// Without the acronym table this would be "get-t-v-d-b-episodes".
			name:      "TVDB acronym is repaired",
			handler:   "HandleGetTVDBEpisodes",
			groupName: "METADATA",
			want:      "METADATA-get-tvdb-episodes",
		},
		{
			name:      "MAL acronym is repaired",
			handler:   "HandleMALAuth",
			groupName: "MAL",
			want:      "MAL-mal-auth",
		},
		{
			name:      "digits do not introduce a dash",
			handler:   "HandleGetV2Thing",
			groupName: "G",
			want:      "G-get-v2-thing",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, getEndpointKey(tc.handler, tc.groupName))
		})
	}
}

func TestStringGoTypeToTypescriptType(t *testing.T) {
	cases := []struct{ in, want string }{
		// Scalars.
		{"string", "string"},
		{"int", "number"},
		{"float64", "number"},
		{"bool", "boolean"},
		{"nil", "null"},
		{"time.Time", "string"},
		// Free-form JSON.
		{"json.RawMessage", "Record<string, any>"},
		{"RawMessage", "Record<string, any>"},
		// Composites.
		{"[]string", "Array<string>"},
		{"*string", "string"},
		{"[]*string", "Array<string>"},
		{"map[string]int", "Record<string, number>"},
		// Qualified types get their package's prefix.
		{"models.User", "Models_User"},
		{"[]models.User", "Array<Models_User>"},
		{"[]*models.User", "Array<Models_User>"},
		{"map[string]models.User", "Record<string, Models_User>"},
		{"map[string][]*models.User", "Record<string, Array<Models_User>>"},
		// An unqualified, unknown type is returned unchanged.
		{"SomeLocalType", "SomeLocalType"},
		// Deliberately NOT mapped here, unlike fieldTypeToTypescriptType.
		{"byte", "byte"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, stringGoTypeToTypescriptType(tc.in))
		})
	}
}

func TestGoTypeToTypescriptType(t *testing.T) {
	t.Run("scalars", func(t *testing.T) {
		cases := map[string]string{
			"string": "string", "int": "number", "float32": "number",
			"bool": "boolean", "nil": "null", "time.Time": "string",
		}
		for in, want := range cases {
			t.Run(in, func(t *testing.T) {
				require.Equal(t, want, goTypeToTypescriptType(in))
			})
		}
	})

	t.Run("anything else is unknown", func(t *testing.T) {
		// "unknown" is how isCustomStruct recognizes a struct, so this fallback
		// is behavior, not a placeholder.
		for _, in := range []string{"models.User", "[]string", "byte", "json.RawMessage", ""} {
			t.Run(in, func(t *testing.T) {
				require.Equal(t, "unknown", goTypeToTypescriptType(in))
			})
		}
	})
}

func TestIsCustomStruct(t *testing.T) {
	require.True(t, isCustomStruct("models.User"))
	require.True(t, isCustomStruct("SomeType"))
	require.False(t, isCustomStruct("string"))
	require.False(t, isCustomStruct("int"))
	require.False(t, isCustomStruct("bool"))
	require.False(t, isCustomStruct("time.Time"))
}

func TestGetUnformattedGoType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"models.User", "models.User"},
		{"[]models.User", "models.User"},
		{"*models.User", "models.User"},
		{"[]*models.User", "models.User"},
		{"map[string]models.User", "models.User"},
		{"map[string][]*models.User", "models.User"},
		{"string", "string"},
		{"true", "true"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, getUnformattedGoType(tc.in))
		})
	}
}

func TestFieldTypeToUsedTypescriptType(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain type is itself", "Models_User", "Models_User"},
		{"array is unwrapped", "Array<Models_User>", "Models_User"},
		{"nested arrays are unwrapped", "Array<Array<Models_User>>", "Models_User"},
		{"record yields its value type", "Record<string, Models_User>", "Models_User"},
		{
			// Exercises the bracket-depth scan: the first comma inside Array<>
			// must not be mistaken for the record's key/value separator.
			name: "record of array unwraps to the element",
			in:   "Record<string, Array<Models_User>>",
			want: "Models_User",
		},
		{"record of record", "Record<string, Record<string, Models_User>>", "Models_User"},
		{"primitives have no used type", "string", ""},
		{"number", "number", ""},
		{"boolean", "boolean", ""},
		{"any", "any", ""},
		{"null", "null", ""},
		{"undefined", "undefined", ""},
		{"array of primitives", "Array<string>", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, fieldTypeToUsedTypescriptType(tc.in))
		})
	}
}

func TestToPascalCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"playback_manager", "PlaybackManager"},
		{"single", "Single"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, toPascalCase(tc.in))
		})
	}
}
