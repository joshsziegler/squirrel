package templates

import (
	"fmt"
	"io"
)

// ShortWriter allows simplified use of fmt.Fprintln() and fmt.Fprintf with an
// io.Writer to make the code easier to read, especially large blocks.
type ShortWriter struct {
	w io.Writer
}

func NewShortWriter(w io.Writer) *ShortWriter {
	return &ShortWriter{w: w}
}

// N ~ Newline
func (x *ShortWriter) N(s string) {
	fmt.Fprintln(x.w, s)
}

// F ~ Format
func (x *ShortWriter) F(format string, a ...any) {
	fmt.Fprintf(x.w, format, a...)
}
