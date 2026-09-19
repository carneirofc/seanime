package codegen

import (
	"fmt"
	"io"
)

// errWriter is a thin io.Writer wrapper that remembers the first write error and
// then does nothing, so emitters can stay readable.
//
// The generators emit hundreds of small fragments; checking every single write
// inline would bury the shape of the output being produced. Instead, writes are
// issued unchecked and the caller inspects Err() once, after the last one.
//
// It exists for two reasons:
//
//  1. Errors. Before this, every WriteString return value in codegen was
//     discarded, so a full disk or a read-only path produced a truncated
//     generated file and a successful exit.
//  2. Testability. Emitters take an *errWriter rather than a concrete *os.File,
//     so tests can render into a bytes.Buffer and assert on the result.
type errWriter struct {
	w   io.Writer
	err error
}

func newErrWriter(w io.Writer) *errWriter {
	return &errWriter{w: w}
}

// WriteString writes s unless a previous write already failed.
func (e *errWriter) WriteString(s string) {
	if e.err != nil {
		return
	}
	_, e.err = io.WriteString(e.w, s)
}

// Printf formats and writes, unless a previous write already failed.
func (e *errWriter) Printf(format string, args ...any) {
	e.WriteString(fmt.Sprintf(format, args...))
}

// Err returns the first write error, if any.
func (e *errWriter) Err() error {
	return e.err
}
