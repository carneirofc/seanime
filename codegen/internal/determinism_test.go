package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The CI job "Codegen up to date" regenerates everything and fails if the tree
// moved. That only works while the generator is deterministic, and several stages
// iterate Go maps -- whose order is randomized per run -- before emitting:
// structStrMap and structsByPackage in GenerateTypescriptFile, groupedByFile and
// endpointsMap in GenerateTypescriptEndpointsFile. Each is sorted before being
// written today. These tests are what keep it that way; without them a dropped
// sort would show up as an intermittently red CI job on unrelated pull requests.

// readAll returns every regular file under dir, keyed by path relative to dir.
func readAll(t *testing.T, dir string) map[string]string {
	t.Helper()

	out := map[string]string{}
	require.NoError(t, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	}))
	return out
}

func TestExtractStructsIsDeterministic(t *testing.T) {
	src := filepath.Join("testdata", "structs")

	first, second := t.TempDir(), t.TempDir()
	require.NoError(t, ExtractStructs(src, first))
	require.NoError(t, ExtractStructs(src, second))

	require.Equal(t, readAll(t, first), readAll(t, second),
		"two runs over the same input must produce identical bytes")
}

func TestGenerateHandlersIsDeterministic(t *testing.T) {
	src := filepath.Join("testdata", "handlers")

	first, second := t.TempDir(), t.TempDir()
	require.NoError(t, GenerateHandlers(src, first))
	require.NoError(t, GenerateHandlers(src, second))

	require.Equal(t, readAll(t, first), readAll(t, second))
}

// TestTypescriptPipelineIsDeterministic runs the two TypeScript stages back to
// back, which is where the map iteration actually matters.
func TestTypescriptPipelineIsDeterministic(t *testing.T) {
	run := func(t *testing.T) map[string]string {
		t.Helper()

		jsonDir := t.TempDir()
		require.NoError(t, GenerateHandlers(filepath.Join("testdata", "handlers"), jsonDir))
		require.NoError(t, ExtractStructs(filepath.Join("testdata", "structs"), jsonDir))

		handlersJson := filepath.Join(jsonDir, "handlers.json")
		structsJson := filepath.Join(jsonDir, "public_structs.json")

		webDir := t.TempDir()
		eventsDir := t.TempDir()

		goStructStrs, err := GenerateTypescriptEndpointsFile(handlersJson, structsJson, webDir, eventsDir)
		require.NoError(t, err)
		require.NoError(t, GenerateTypescriptFile(handlersJson, structsJson, webDir, goStructStrs))

		out := readAll(t, webDir)
		for k, v := range readAll(t, eventsDir) {
			out["events/"+k] = v
		}
		return out
	}

	// Several iterations, because a map-ordering bug is probabilistic: with only
	// two runs a two-element map agrees half the time.
	baseline := run(t)
	for i := 0; i < 8; i++ {
		require.Equal(t, baseline, run(t), "run %d differed from the first", i+2)
	}
}

func TestGeneratedTypescriptCarriesTheDoNotEditBanner(t *testing.T) {
	jsonDir := t.TempDir()
	require.NoError(t, GenerateHandlers(filepath.Join("testdata", "handlers"), jsonDir))
	require.NoError(t, ExtractStructs(filepath.Join("testdata", "structs"), jsonDir))

	webDir := t.TempDir()
	goStructStrs, err := GenerateTypescriptEndpointsFile(
		filepath.Join(jsonDir, "handlers.json"),
		filepath.Join(jsonDir, "public_structs.json"),
		webDir, t.TempDir())
	require.NoError(t, err)
	require.NoError(t, GenerateTypescriptFile(
		filepath.Join(jsonDir, "handlers.json"),
		filepath.Join(jsonDir, "public_structs.json"),
		webDir, goStructStrs))

	for _, name := range []string{typescriptFileName, typescriptEndpointsFileName, typescriptEndpointTypesFileName} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(webDir, name))
			require.NoError(t, err)
			require.Contains(t, string(b), "DO NOT EDIT",
				"generated files must say so; AGENTS.md tells contributors never to hand-edit them")
		})
	}
}

func TestLoadersReportMissingAndMalformedInput(t *testing.T) {
	t.Run("missing handlers file", func(t *testing.T) {
		_, err := LoadHandlers(filepath.Join(t.TempDir(), "nope.json"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "reading handlers")
	})

	t.Run("missing structs file", func(t *testing.T) {
		_, err := LoadPublicStructs(filepath.Join(t.TempDir(), "nope.json"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "reading public structs")
	})

	t.Run("malformed handlers file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "handlers.json")
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))

		_, err := LoadHandlers(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "parsing handlers")
	})

	t.Run("malformed structs file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "structs.json")
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))

		_, err := LoadPublicStructs(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "parsing public structs")
	})
}

func TestExtractStructsReportsAWalkFailure(t *testing.T) {
	// A directory that does not exist must fail, not silently produce an empty
	// contract file. This is the case that used to print to stdout and exit 0.
	err := ExtractStructs(filepath.Join(t.TempDir(), "does-not-exist"), t.TempDir())
	require.Error(t, err)
}
