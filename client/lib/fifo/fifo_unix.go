// +build !windows

package fifo

import (
	"context"
	"io"
	"os"
	"syscall"

	cfifo "github.com/containerd/fifo"
)

// New creates a fifo at the given path and returns an io.ReadWriteCloser for it
// The fifo must not already exist
func New(path string) (io.ReadWriteCloser, error) {
	return cfifo.OpenFifo(context.Background(), path, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0600)
}

// Open opens a fifo that already exists and returns an io.ReadWriteCloser for it
func Open(path string) (io.ReadWriteCloser, error) {
	return cfifo.OpenFifo(context.Background(), path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0600)
}

// Remove a fifo that already exists at a given path
func Remove(path string) error {
	return os.Remove(path)
}

func IsClosedErr(err error) bool {
	err2, ok := err.(*os.PathError)
	if ok {
		return err2.Err == os.ErrClosed
	}
	return false
}
