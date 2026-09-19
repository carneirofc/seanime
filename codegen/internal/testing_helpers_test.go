package codegen

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stringBuilder is a minimal io.Writer that collects output, used instead of
// bytes.Buffer so the intent at each call site is obvious.
type stringBuilder struct{ sb strings.Builder }

func (s *stringBuilder) Write(p []byte) (int, error) { return s.sb.Write(p) }
func (s *stringBuilder) String() string              { return s.sb.String() }

// parseTypeExpr parses a Go type expression, e.g. "map[string]*models.User".
func parseTypeExpr(t *testing.T, src string) ast.Expr {
	t.Helper()

	expr, err := parser.ParseExpr(src)
	require.NoErrorf(t, err, "parsing type expression %q", src)
	return expr
}

// parseField parses a single struct field declaration, e.g.
//
//	Name string `json:"name,omitempty"`
//
// It returns the *ast.Field so tag- and comment-sensitive helpers can be tested.
func parseField(t *testing.T, src string) *ast.Field {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "field.go",
		"package p\ntype s struct {\n"+src+"\n}\n", parser.ParseComments)
	require.NoErrorf(t, err, "parsing field %q", src)

	structType := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
	require.Len(t, structType.Fields.List, 1, "expected exactly one field in %q", src)
	return structType.Fields.List[0]
}

// parseFuncDecl parses a single function declaration so parseBodyFields can be
// exercised without touching the filesystem.
func parseFuncDecl(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "fn.go", "package p\n"+src, parser.ParseComments)
	require.NoError(t, err)

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatalf("no function declaration found in %q", src)
	return nil
}

// mustStructType asserts expr is an anonymous struct type and returns it.
func mustStructType(t *testing.T, expr ast.Expr) *ast.StructType {
	t.Helper()

	st, ok := expr.(*ast.StructType)
	require.True(t, ok, "expected a struct type, got %T", expr)
	return st
}

// osWriteFile writes content to path, creating it if needed.
func osWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// failingWriter fails every write, to check errWriter latches the error.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errAlwaysFails }

var errAlwaysFails = errors.New("write failed")

// readFileString reads a generated file produced into a temp dir by a test.
func readFileString(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}
