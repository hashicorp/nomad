package procio

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/hashicorp/nomad/client/lib/fifo"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 32<<10)
		return &buffer
	},
}

type SetWritersCB func(out, err io.WriteCloser)

// IO is an interface around process IO. Currently this includes stdout and
// stderr.
type IO interface {
	io.Closer
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Set(SetWritersCB)
}

type IOType string

const (
	IOTypeEmpty  IOType = ""
	IOTypeFIFO   IOType = "fifo"
	IOTypeBuffer IOType = "buffer"
)

// Stdio is the configuration for where to write standard process output to
type Stdio struct {
	IOType IOType
	Stdout string
	Stderr string
}

// IsNull returns true if the stdio is not defined
func (s Stdio) IsNull() bool {
	return s.IOType == "" && s.Stdout == "" && s.Stderr == ""
}

// ProcessIO wraps an IO interface and and sends output to the configured
// destination
type ProcessIO struct {
	io IO

	copyLoop bool
	stdio    Stdio
}

func NewProcessIO(stdio Stdio, cb SetWritersCB) (pio *ProcessIO, err error) {
	pio = &ProcessIO{
		stdio:    stdio,
		copyLoop: true,
	}

	if stdio.IsNull() {
		i, err := NewNullIO()
		if err != nil {
			return nil, err
		}
		pio.io = i
		return pio, nil
	}

	switch stdio.IOType {

	case IOTypeFIFO, IOTypeEmpty:
		pio.io, err = NewPipeIO()
		if err != nil {
			return nil, err
		}
	case IOTypeBuffer:
		pio.io = NewBufferIO()
		pio.copyLoop = false
	}
	pio.io.Set(cb)

	return pio, nil
}

func (p *ProcessIO) Close() error {
	if p.io != nil {
		return p.io.Close()
	}
	return nil
}

func (p *ProcessIO) IO() IO {
	return p.io
}

func (p *ProcessIO) Copy(wg *sync.WaitGroup) error {
	if !p.copyLoop {
		return nil
	}

	if err := copyLoop(p.stdio.Stdout, p.io.Stdout(), wg); err != nil {
		return fmt.Errorf("failed to start copy loop for process stdout: %v", err)
	}

	if err := copyLoop(p.stdio.Stderr, p.io.Stderr(), wg); err != nil {
		return fmt.Errorf("failed to start copy loop for process stderr: %v", err)
	}

	return nil
}

func copyLoop(path string, r io.ReadCloser, wg *sync.WaitGroup) error {
	fw, err := fifo.Open(path)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		p := bufPool.Get().(*[]byte)
		defer bufPool.Put(p)
		io.CopyBuffer(fw, r, *p)
		wg.Done()
		fw.Close()
	}()

	return nil
}

// NewNullIO returns IO setup for /dev/null use
func NewNullIO() (IO, error) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	return &nullIO{
		devNull: f,
	}, nil
}

type nullIO struct {
	devNull *os.File
}

func (n *nullIO) Close() error {
	n.devNull.Close()
	return nil
}

func (n *nullIO) Stdout() io.ReadCloser {
	return nil
}

func (n *nullIO) Stderr() io.ReadCloser {
	return nil
}

func (n *nullIO) Set(f SetWritersCB) {
	f(n.devNull, n.devNull)
}
