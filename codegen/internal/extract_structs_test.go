package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// structsFromFixture parses one file under testdata/structs and indexes the result
// by type name.
func structsFromFixture(t *testing.T, name string) map[string]*GoStruct {
	t.Helper()

	path := filepath.Join("testdata", "structs", name)
	info, err := os.Stat(path)
	require.NoError(t, err)

	got, err := getGoStructsFromFile(path, info)
	require.NoError(t, err)

	byName := make(map[string]*GoStruct, len(got))
	for _, s := range got {
		byName[s.Name] = s
	}
	return byName
}

// fieldByName finds a field on a struct, failing the test when it is absent.
func fieldByName(t *testing.T, s *GoStruct, name string) *GoStructField {
	t.Helper()

	for _, f := range s.Fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("struct %s has no field %s", s.Name, name)
	return nil
}

func TestGetGoStructsFromFile_PlainStruct(t *testing.T) {
	structs := structsFromFixture(t, "plain.go")

	require.Contains(t, structs, "User")
	require.NotContains(t, structs, "unexportedType", "unexported types must not be extracted")

	user := structs["User"]
	require.Equal(t, "fixtures", user.Package)
	require.Equal(t, "plain.go", user.Filename)
	require.Equal(t, "User", user.FormattedName, "package 'fixtures' has no prefix configured")
	require.Equal(t, []string{" User is a person.", " Second comment line."}, user.Comments)

	t.Run("json names and required flags", func(t *testing.T) {
		cases := []struct {
			field    string
			jsonName string
			required bool
			tsType   string
		}{
			// A plain scalar with no omitempty is required.
			{"ID", "id", true, "number"},
			{"Name", "name", true, "string"},
			// omitempty makes it optional.
			{"Nickname", "nickname", false, "string"},
			// A pointer is optional regardless of omitempty.
			{"Avatar", "avatar", false, "string"},
			// A slice is optional regardless of omitempty.
			{"Tags", "tags", false, "Array<string>"},
			// []byte: see TestByteSliceGoTypeAndTypescriptTypeDisagree below --
			// GoType becomes "string" but TypescriptType becomes "Array<string>".
			{"Data", "data", false, "Array<string>"},
			// No tag at all: the Go field name is used verbatim.
			{"NoTag", "NoTag", true, "boolean"},
		}
		for _, tc := range cases {
			t.Run(tc.field, func(t *testing.T) {
				f := fieldByName(t, user, tc.field)
				require.Equal(t, tc.jsonName, f.JsonName)
				require.Equal(t, tc.required, f.Required)
				require.Equal(t, tc.tsType, f.TypescriptType)
			})
		}
	})

	t.Run(`json:"-" yields an empty JsonName so emitters skip it`, func(t *testing.T) {
		require.Empty(t, fieldByName(t, user, "Secret").JsonName)
	})

	t.Run("unexported fields are extracted but marked non-public", func(t *testing.T) {
		require.False(t, fieldByName(t, user, "unexported").Public)
		require.True(t, fieldByName(t, user, "Name").Public)
	})
}

func TestGetGoStructsFromFile_EmbeddedStruct(t *testing.T) {
	structs := structsFromFixture(t, "embedded.go")

	derived := structs["Derived"]
	require.Equal(t, []string{"fixtures.Base"}, derived.EmbeddedStructTypes,
		"an embedded type is recorded for later expansion, not flattened here")

	// The embedded type contributes no field of its own at extraction time.
	require.Len(t, derived.Fields, 1)
	require.Equal(t, "own", derived.Fields[0].JsonName)
}

func TestGetGoStructsFromFile_Aliases(t *testing.T) {
	structs := structsFromFixture(t, "alias.go")

	t.Run("string alias collects exported declared values as a union", func(t *testing.T) {
		status := structs["Status"]
		require.NotNil(t, status.AliasOf)
		require.Equal(t, "string", status.AliasOf.GoType)
		require.Equal(t, "string", status.AliasOf.TypescriptType)
		require.Equal(t, []string{`"active"`, `"inactive"`}, status.AliasOf.DeclaredValues,
			"unexported constants must not become part of the union")
	})

	t.Run("self-referential alias is skipped", func(t *testing.T) {
		require.NotContains(t, structs, "SelfAlias")
	})

	t.Run("map alias", func(t *testing.T) {
		lookup := structs["Lookup"]
		require.NotNil(t, lookup.AliasOf)
		require.Equal(t, "map[string]User", lookup.AliasOf.GoType)
		require.Equal(t, "Record<string, User>", lookup.AliasOf.TypescriptType)
		require.Equal(t, "fixtures.User", lookup.AliasOf.UsedStructType)
	})

	t.Run("slice alias", func(t *testing.T) {
		list := structs["List"]
		require.NotNil(t, list.AliasOf)
		require.Equal(t, "[]User", list.AliasOf.GoType)
		require.Equal(t, "Array<User>", list.AliasOf.TypescriptType)
		require.Equal(t, "fixtures.User", list.AliasOf.UsedStructType)
	})

	t.Run("numeric alias has no declared values and no used struct", func(t *testing.T) {
		count := structs["Count"]
		require.NotNil(t, count.AliasOf)
		require.Equal(t, "number", count.AliasOf.TypescriptType)
		require.Empty(t, count.AliasOf.DeclaredValues)
		require.Empty(t, count.AliasOf.UsedStructType)
	})
}

func TestGetGoStructsFromFile_InlineStructBecomesSyntheticType(t *testing.T) {
	structs := structsFromFixture(t, "inline.go")

	// An anonymous struct field is lifted into its own synthetic type named
	// Parent_Nested, and the parent field is repointed at it.
	require.Contains(t, structs, "Parent_Nested", "inline struct should be lifted to its own type")

	nested := structs["Parent_Nested"]
	require.Equal(t, "enabled", nested.Fields[0].JsonName)
	require.Equal(t, "numbers", nested.Fields[1].JsonName)

	field := fieldByName(t, structs["Parent"], "Nested")
	require.Equal(t, "Parent_Nested", field.GoType, "the __STRUCT__ placeholder must be resolved")
	require.Equal(t, "Parent_Nested", field.TypescriptType)
	require.Equal(t, "fixtures.Parent_Nested", field.UsedStructType)
}

func TestGetGoStructsFromFile_UsesForwardSlashPaths(t *testing.T) {
	structs := structsFromFixture(t, "plain.go")

	// Filepath is emitted into TypeScript doc comments, so it must not vary by OS.
	require.Equal(t, "testdata/structs/plain.go", structs["User"].Filepath)
}

func TestGetGoStructsFromFile_ReturnsErrorOnUnparseableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go")
	require.NoError(t, os.WriteFile(path, []byte("package p\ntype ("), 0o644))

	info, err := os.Stat(path)
	require.NoError(t, err)

	_, err = getGoStructsFromFile(path, info)
	require.Error(t, err, "a syntax error must surface, not be silently skipped")
}

// TestByteSliceGoTypeAndTypescriptTypeDisagree pins a known inconsistency rather
// than endorsing it.
//
// fieldTypeString maps []byte to "string", but fieldTypeToTypescriptType emits
// "Array<string>". Its guard is unreachable:
//
//	if fieldTypeToTypescriptType(t.Elt, pkg) == "byte" { return "string" }
//
// fieldTypeToTypescriptType never returns "byte" -- it maps byte to "string" --
// so the comparison is always false. The equivalent guard in fieldTypeString
// works because fieldTypeString does return "byte".
//
// Making the guard effective would retype 41 committed frontend fields from
// Array<string> to string, so it is left alone deliberately. This test exists so
// the change is a deliberate one when someone makes it: if it starts failing,
// that is the fix landing, and the generated frontend types must be regenerated
// and typechecked in the same change.
func TestByteSliceGoTypeAndTypescriptTypeDisagree(t *testing.T) {
	expr := parseTypeExpr(t, "[]byte")

	require.Equal(t, "string", fieldTypeString(expr),
		"fieldTypeString collapses []byte to string")
	require.Equal(t, "Array<string>", fieldTypeToTypescriptType(expr, ""),
		"fieldTypeToTypescriptType does not, because its byte guard is unreachable")
}
