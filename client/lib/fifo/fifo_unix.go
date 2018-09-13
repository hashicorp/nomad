// +build !windows

package fifo

import (
	"context"
	"io"
	"os"
	"syscall"

	cfifo "github.com/containerd/fifo"
)

// New creates a fifo at the given path and returns a reader for it
// The fifo must not already exist
func New(path string) (io.ReadCloser, error) {
	return cfifo.OpenFifo(context.Background(), path, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0600)
}

// OpenWriter opens a fifo that already exists and returns a writer for it
func OpenWriter(path string) (io.WriteCloser, error) {
	return cfifo.OpenFifo(context.Background(), path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0600)
}

// Remove a fifo that already exists at a given path
func Remove(path string) error {
	return os.Remove(path)
}
