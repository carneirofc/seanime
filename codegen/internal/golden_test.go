package codegen

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// update rewrites the golden files instead of asserting against them:
//
//	go test ./codegen/internal -run TestEmit -update
//
// Review the resulting diff before committing it — a golden file is only useful
// while someone still reads what changed in it.
var update = flag.Bool("update", false, "update golden files in testdata/golden")

// assertGolden compares got against testdata/golden/<name>, or rewrites it when
// -update is set.
func assertGolden(t *testing.T, name string, got string) {
	t.Helper()

	path := filepath.Join("testdata", "golden", name)

	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		t.Logf("updated golden file %s", path)
		return
	}

	want, err := os.ReadFile(path)
	require.NoErrorf(t, err, "missing golden file %s; run with -update to create it", path)
	require.Equal(t, string(want), got, "generated output does not match %s; run with -update if the change is intended", path)
}

// render runs an emitter against a buffer and returns what it wrote, failing the
// test if the emitter reported a write error.
func render(t *testing.T, emit func(w *errWriter)) string {
	t.Helper()

	var sb stringBuilder
	w := newErrWriter(&sb)
	emit(w)
	require.NoError(t, w.Err())
	return sb.String()
}
