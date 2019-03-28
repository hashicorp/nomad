package procio

import (
	"io"
	"os"
)

func newPipe() (*pipe, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &pipe{
		r: r,
		w: w,
	}, nil
}

type pipe struct {
	r *os.File
	w *os.File
}

func (p *pipe) Close() error {
	err := p.w.Close()
	if rerr := p.r.Close(); err == nil {
		err = rerr
	}
	return err
}

type pipeIO struct {
	out *pipe
	err *pipe
}

func NewPipeIO() (i IO, err error) {

	var (
		pipes          []*pipe
		stdout, stderr *pipe
	)

	// cleanup in case of an error
	defer func() {
		if err != nil {
			for _, p := range pipes {
				p.Close()
			}
		}
	}()

	// setup stdout pipe and fifo
	if stdout, err = newPipe(); err != nil {
		return nil, err
	}
	pipes = append(pipes, stdout)

	// setup stderr pipe and fifo
	if stderr, err = newPipe(); err != nil {
		return nil, err
	}
	pipes = append(pipes, stderr)

	return &pipeIO{
		out: stdout,
		err: stderr,
	}, nil
}

func (i *pipeIO) Close() error {
	var err error
	for _, p := range []*pipe{
		i.out,
		i.err,
	} {
		if p != nil {
			// capture first close error if occurs
			if closeErr := p.Close(); err == nil {
				err = closeErr
			}
		}
	}
	return err
}

func (i *pipeIO) Stdout() io.ReadCloser {
	return i.out.r
}

func (i *pipeIO) Stderr() io.ReadCloser {
	return i.err.r
}

func (i *pipeIO) Set(f SetWritersCB) {
	f(i.out.w, i.err.w)
}
