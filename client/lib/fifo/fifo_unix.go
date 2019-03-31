// +build !windows

package fifo

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// New creates a fifo at the given path, and returns an open function for reading.
// The fifo must not exist already, or that it's already a fifo file
//
// It returns a reader open function that may block until a writer opens
// so it's advised to run it in a goroutine different from reader goroutine
func New(path string) (func() (io.ReadCloser, error), error) {
	// create first
	if err := mkfifo(path, 0600); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("error creating fifo %v: %v", path, err)
	}

	openFn := func() (io.ReadCloser, error) {
		return os.OpenFile(path, unix.O_RDONLY, os.ModeNamedPipe)
	}

	return openFn, nil
}

// Open opens a fifo file for reading, assuming it already exists, returns io.WriteCloser
func Open(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, unix.O_WRONLY, os.ModeNamedPipe)
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

func mkfifo(path string, mode uint32) (err error) {
	return unix.Mkfifo(path, mode)
}
