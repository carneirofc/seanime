package codegen

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// structFixture builds a GoStruct whose fields reference the given types.
func structFixture(pkg, name string, referencedTypes ...string) *GoStruct {
	s := &GoStruct{
		Package:       pkg,
		Name:          name,
		FormattedName: getTypePrefix(pkg) + name,
		Fields:        make([]*GoStructField, 0, len(referencedTypes)),
	}
	for _, ref := range referencedTypes {
		s.Fields = append(s.Fields, &GoStructField{
			Name:           ref,
			JsonName:       ref,
			UsedStructType: ref,
		})
	}
	return s
}

// keysOf returns the sorted keys of a resolved struct set.
func keysOf(m map[string]*GoStruct) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestGetReferencedStructsRecursively(t *testing.T) {
	t.Run("follows field references transitively", func(t *testing.T) {
		a := structFixture("p", "A", "p.B")
		b := structFixture("p", "B", "p.C")
		c := structFixture("p", "C")

		all := map[string]*GoStruct{"p.A": a, "p.B": b, "p.C": c}

		got, ok := getReferencedStructsRecursively([]*GoStruct{a}, nil, all)
		require.True(t, ok)
		require.Equal(t, []string{"p.A", "p.B", "p.C"}, keysOf(got))
	})

	t.Run("terminates on a reference cycle", func(t *testing.T) {
		// A -> B -> A. Without the visited-set guard this recurses forever, so
		// this test is the reason that guard exists.
		a := structFixture("p", "A", "p.B")
		b := structFixture("p", "B", "p.A")

		all := map[string]*GoStruct{"p.A": a, "p.B": b}

		got, ok := getReferencedStructsRecursively([]*GoStruct{a}, nil, all)
		require.True(t, ok)
		require.Equal(t, []string{"p.A", "p.B"}, keysOf(got))
	})

	t.Run("terminates on a self reference", func(t *testing.T) {
		a := structFixture("p", "A", "p.A")
		all := map[string]*GoStruct{"p.A": a}

		got, ok := getReferencedStructsRecursively([]*GoStruct{a}, nil, all)
		require.True(t, ok)
		require.Equal(t, []string{"p.A"}, keysOf(got))
	})

	t.Run("a reference to an unknown type is ignored", func(t *testing.T) {
		// Types outside the scanned tree (stdlib, third-party) legitimately
		// have no entry, so a dangling reference must not fail the run.
		a := structFixture("p", "A", "elsewhere.Missing")
		all := map[string]*GoStruct{"p.A": a}

		got, ok := getReferencedStructsRecursively([]*GoStruct{a}, nil, all)
		require.True(t, ok)
		require.Equal(t, []string{"p.A"}, keysOf(got))
	})

	t.Run("follows an alias to the type it wraps", func(t *testing.T) {
		list := &GoStruct{
			Package: "p", Name: "List", FormattedName: "List",
			Fields:  []*GoStructField{},
			AliasOf: &GoAlias{GoType: "[]User", UsedStructType: "p.User"},
		}
		user := structFixture("p", "User")

		all := map[string]*GoStruct{"p.List": list, "p.User": user}

		got, ok := getReferencedStructsRecursively([]*GoStruct{list}, nil, all)
		require.True(t, ok)
		require.Equal(t, []string{"p.List", "p.User"}, keysOf(got))
	})

	t.Run("merges the shared and other sets and de-duplicates", func(t *testing.T) {
		shared := structFixture("p", "Shared", "p.Common")
		other := structFixture("p", "Other", "p.Common")
		common := structFixture("p", "Common")

		all := map[string]*GoStruct{"p.Shared": shared, "p.Other": other, "p.Common": common}

		got, ok := getReferencedStructsRecursively([]*GoStruct{shared}, []*GoStruct{other}, all)
		require.True(t, ok)
		require.Equal(t, []string{"p.Common", "p.Other", "p.Shared"}, keysOf(got))
	})

	t.Run("a field with no struct reference contributes nothing", func(t *testing.T) {
		a := &GoStruct{
			Package: "p", Name: "A", FormattedName: "A",
			Fields: []*GoStructField{{Name: "Count", JsonName: "count", TypescriptType: "number"}},
		}
		all := map[string]*GoStruct{"p.A": a}

		got, ok := getReferencedStructsRecursively([]*GoStruct{a}, nil, all)
		require.True(t, ok)
		require.Equal(t, []string{"p.A"}, keysOf(got))
	})
}
