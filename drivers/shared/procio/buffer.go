package procio

import (
	"bytes"
	"io"
)

type bufferCloser struct {
	bytes.Buffer
}

func (_ *bufferCloser) Close() error { return nil }

type BufferIO struct {
	out *bufferCloser
	err *bufferCloser
}

func NewBufferIO() IO {
	return &BufferIO{out: &bufferCloser{}, err: &bufferCloser{}}
}

func (b *BufferIO) Close() error {
	return nil
}

func (b *BufferIO) Stdout() io.ReadCloser {
	return b.out
}

func (b *BufferIO) Stderr() io.ReadCloser {
	return b.err
}

func (b *BufferIO) Set(f SetWritersCB) {
	f(b.out, b.err)
}

func (b *BufferIO) Buffers() (out *bytes.Buffer, err *bytes.Buffer) {
	return &b.out.Buffer, &b.err.Buffer
}
