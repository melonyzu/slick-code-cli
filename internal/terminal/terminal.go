package terminal

import (
	"bufio"
	"io"
	"os"
)

// Terminal is the CLI's IO surface: the streams commands read from and
// write to. Injecting it, rather than reaching for os.Stdin/os.Stdout/
// os.Stderr directly, gives the runtime a single place to control
// input/output and keeps commands testable.
type Terminal struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	reader *bufio.Reader
}

// New returns a Terminal backed by the process's standard streams.
func New() *Terminal {
	return &Terminal{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}
}

// Reader returns a single buffered reader over In. Every line-based
// read goes through it so that bytes buffered by one prompt are not
// lost to the next consumer — including the interactive UI, which must
// use this same reader when In is not a real terminal.
func (t *Terminal) Reader() *bufio.Reader {
	if t.reader == nil {
		t.reader = bufio.NewReader(t.In)
	}
	return t.reader
}
