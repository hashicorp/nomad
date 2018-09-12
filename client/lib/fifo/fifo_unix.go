// +build !windows

package fifo

import (
	"io"
	"os"
	"syscall"
)

// New creates a fifo at the given path and returns a reader for it
// The fifo must not already exist
func New(path string) (io.ReadCloser, error) {
	err := syscall.Mkfifo(path, 0600)
	if err != nil {
		return nil, err
	}

	return open(path)
}

func open(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenWriter opens a fifo that already exists and returns a writer for it
func OpenWriter(path string) (io.WriteCloser, error) {
	return open(path)
}

// Remove a fifo that already exists at a given path
func Remove(path string) error {
	return os.Remove(path)
}
