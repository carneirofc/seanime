package codegen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFieldTypeString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"string", "string"},
		{"models.User", "models.User"},
		// Pointers are erased: the generated TypeScript expresses optionality
		// through the Required flag instead.
		{"*models.User", "models.User"},
		{"[]models.User", "[]models.User"},
		{"[]*models.User", "[]models.User"},
		{"map[string]models.User", "map[string]models.User"},
		{"map[string][]*models.User", "map[string][]models.User"},
		// []byte is a string, not an array of bytes.
		{"[]byte", "string"},
		// An anonymous struct is a placeholder to be resolved by the caller.
		{"struct{ A string }", "__STRUCT__"},
		// Unsupported expressions collapse to empty.
		{"func() error", ""},
		{"chan int", ""},
		{"interface{}", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, fieldTypeString(parseTypeExpr(t, tc.in)))
		})
	}
}

func TestFieldTypeToTypescriptType(t *testing.T) {
	cases := []struct {
		name string
		in   string
		pkg  string
		want string
	}{
		{"string", "string", "", "string"},
		{"int", "int", "", "number"},
		{"bool", "bool", "", "boolean"},
		{"byte", "byte", "", "string"},
		{"pointer is unwrapped", "*string", "", "string"},
		{"slice", "[]string", "", "Array<string>"},
		{"map", "map[string]int", "", "Record<string, number>"},
		{"qualified type gets its package prefix", "models.User", "models", "Models_User"},
		{"slice of qualified type", "[]models.User", "models", "Array<Models_User>"},
		{"pointer to qualified type", "*models.User", "models", "Models_User"},
		{"time.Time is a string", "time.Time", "time", "string"},
		{"slice of time.Time", "[]time.Time", "time", "Array<string>"},
		{"unprefixed package is left bare", "unknownpkg.Thing", "unknownpkg", "Thing"},
		{"anonymous struct is inlined", "struct{ A string }", "", "{ A: string; }"},
		{"unsupported expression is any", "func() error", "", "any"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, fieldTypeToTypescriptType(parseTypeExpr(t, tc.in), tc.pkg))
		})
	}
}

func TestFieldTypeUnformattedString(t *testing.T) {
	cases := []struct{ in, want string }{
		{"models.User", "models.User"},
		{"*models.User", "models.User"},
		{"[]models.User", "models.User"},
		{"[]*models.User", "models.User"},
		// Caveat documented on the function: only the map's value is considered.
		{"map[string]models.User", "models.User"},
		{"string", "string"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, fieldTypeUnformattedString(parseTypeExpr(t, tc.in)))
		})
	}
}

func TestFieldTypeToUsedStructType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"models.User", "models.User"},
		{"*models.User", "models.User"},
		{"[]models.User", "models.User"},
		{"map[string]models.User", "models.User"},
		{"User", "User"},
		{"struct{ A string }", "__STRUCT__"},
		{"func() error", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, fieldTypeToUsedStructType(parseTypeExpr(t, tc.in)))
		})
	}
}

func TestGetUsedStructType(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		pkg     string
		wantTyp string
		wantPkg string
	}{
		{
			name:    "an unqualified type is attributed to the current package",
			in:      "User",
			pkg:     "models",
			wantTyp: "models.User",
			wantPkg: "models",
		},
		{
			name:    "a qualified type keeps its own package",
			in:      "anilist.BaseAnime",
			pkg:     "models",
			wantTyp: "anilist.BaseAnime",
			wantPkg: "anilist",
		},
		{
			name:    "slices are unwrapped first",
			in:      "[]*anilist.BaseAnime",
			pkg:     "models",
			wantTyp: "anilist.BaseAnime",
			wantPkg: "anilist",
		},
		{
			name: "scalars are not struct references",
			in:   "string", pkg: "models", wantTyp: "", wantPkg: "",
		},
		{
			name: "numeric scalars are not struct references",
			in:   "int64", pkg: "models", wantTyp: "", wantPkg: "",
		},
		{
			name: "anonymous structs are not struct references",
			in:   "struct{ A string }", pkg: "models", wantTyp: "", wantPkg: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTyp, gotPkg := getUsedStructType(parseTypeExpr(t, tc.in), tc.pkg)
			require.Equal(t, tc.wantTyp, gotTyp)
			require.Equal(t, tc.wantPkg, gotPkg)
		})
	}
}

func TestJsonFieldName(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"tag name is used", "Name string `json:\"name\"`", "name"},
		{"omitempty is stripped", "Name string `json:\"name,omitempty\"`", "name"},
		{"no tag falls back to the Go name", "Name string", "Name"},
		{"a tag without a json key falls back to the Go name", "Name string `xml:\"name\"`", "Name"},
		{"a dash means the field is not serialized", "Name string `json:\"-\"`", ""},
		{"an empty json name falls back to empty", "Name string `json:\",omitempty\"`", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, jsonFieldName(parseField(t, tc.src)))
		})
	}
}

func TestJsonFieldOmitEmpty(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"omitempty present", "Name string `json:\"name,omitempty\"`", true},
		{"omitempty absent", "Name string `json:\"name\"`", false},
		{"no tag", "Name string", false},
		{"other option is not omitempty", "Name string `json:\"name,string\"`", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, jsonFieldOmitEmpty(parseField(t, tc.src)))
		})
	}
}

func TestFormatInlineStruct(t *testing.T) {
	t.Run("named fields keep their tags, one per line", func(t *testing.T) {
		expr := parseTypeExpr(t, "struct{\nEnabled bool `json:\"enabled\"`\nCount int `json:\"count\"`\n}")
		got := formatInlineStruct(mustStructType(t, expr))
		require.Equal(t, "struct{\nEnabled bool `json:\"enabled\"`\nCount int `json:\"count\"`}", got)
	})

	t.Run("an empty struct still opens a line", func(t *testing.T) {
		expr := parseTypeExpr(t, "struct{}")
		require.Equal(t, "struct{\n}", formatInlineStruct(mustStructType(t, expr)))
	})

	t.Run("an embedded type is rendered without a name", func(t *testing.T) {
		expr := parseTypeExpr(t, "struct{\nmodels.Base\nOwn string `json:\"own\"`\n}")
		got := formatInlineStruct(mustStructType(t, expr))
		require.Equal(t, "struct{\nmodels.Base\nOwn string `json:\"own\"`}", got)
	})
}
