// +build !windows

package fifo

import (
	"io"
	"os"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// New creates a fifo at the given path and returns an os.File for it
// The fifo must not exist already, or that it's already a fifo file
//
// It returns a file open file that may block until a writer opens
// so it's advised to run it in a goroutine different from reader goroutine
func New(path string) (func() (io.ReadCloser, error), error) {
	// create first
	if err := mkfifo(path, 0600); err != nil && !os.IsExist(err) {
		return nil, errors.Wrapf(err, "error creating fifo %v", path)
	}

	openFn := func() (io.ReadCloser, error) {
		return os.OpenFile(path, syscall.O_RDONLY, os.ModeNamedPipe)
	}

	return openFn, nil
}

// Open opens a fifo as a reader that already exists and returns an io.ReadWriteCloser for it
func Open(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, syscall.O_WRONLY, os.ModeNamedPipe)
	io.Copy(nil, nil)
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
