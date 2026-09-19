package codegen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// These tests cover the knobs in config.go. They are what makes the generation
// pattern safe to edit: they pin the behavior of the lookup, not the contents of
// the tables, so adding a package prefix or an acronym does not break them.

func TestGetTypePrefix(t *testing.T) {
	cases := []struct {
		name    string
		pkg     string
		want    string
		comment string
	}{
		{"configured prefix", "models", "Models_", ""},
		{"acronym-style prefix", "anilist", "AL_", ""},
		{"explicitly unprefixed", "handlers", "", "mapped to \"\" on purpose"},
		{"explicitly unprefixed 2", "entities", "", "mapped to \"\" on purpose"},
		{"unknown package", "not_a_real_package", "", "absent packages are unprefixed"},
		{"empty package", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, getTypePrefix(tc.pkg), tc.comment)
		})
	}
}

func TestScalarGoToTS(t *testing.T) {
	t.Run("shared scalars", func(t *testing.T) {
		cases := map[string]string{
			"string":    "string",
			"bool":      "boolean",
			"nil":       "null",
			"time.Time": "string",
			"int":       "number",
			"int8":      "number",
			"int16":     "number",
			"int32":     "number",
			"int64":     "number",
			"uint":      "number",
			"uint8":     "number",
			"uint16":    "number",
			"uint32":    "number",
			"uint64":    "number",
			"float":     "number",
			"float32":   "number",
			"float64":   "number",
		}
		for in, want := range cases {
			t.Run(in, func(t *testing.T) {
				got, ok := scalarGoToTS(in)
				require.True(t, ok)
				require.Equal(t, want, got)
			})
		}
	})

	t.Run("deliberate exclusions", func(t *testing.T) {
		// These are NOT in the shared table; each caller handles them (or does
		// not) for its own reasons. See the comment on scalarGoToTS.
		for _, in := range []string{"byte", "json.RawMessage", "RawMessage", "models.User", "", "rune"} {
			t.Run(in, func(t *testing.T) {
				_, ok := scalarGoToTS(in)
				require.False(t, ok, "%q must not be in the shared scalar table", in)
			})
		}
	})
}

func TestEndpointKeyAcronymsAreLowercaseKebab(t *testing.T) {
	// getEndpointKey applies these after lowercasing and kebab-casing, so an
	// entry with uppercase or underscores would never match anything.
	for _, a := range endpointKeyAcronyms {
		t.Run(a.From, func(t *testing.T) {
			require.Equal(t, a.From, lowerKebab(a.From), "From must be lowercase kebab to ever match")
			require.NotEmpty(t, a.To)
			require.Equal(t, a.To, lowerKebab(a.To))
		})
	}
}

// lowerKebab reports the lowercase form with only letters, digits and dashes.
func lowerKebab(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		}
	}
	return string(out)
}
