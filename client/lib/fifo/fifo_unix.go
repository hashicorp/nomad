// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package fifo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CreateAndRead creates a fifo at the given path, and returns an open function
// for reading. For compatibility with windows, the fifo must not exist
// already.
//
// It returns a reader open function that may block until a writer opens
// so it's advised to run it in a goroutine different from reader goroutine
func CreateAndRead(path string) (func() (io.ReadCloser, error), error) {
	// create first
	if err := mkfifo(path, 0600); err != nil {
		return nil, fmt.Errorf("error creating fifo %v: %w", path, err)
	}

	return func() (io.ReadCloser, error) {
		return OpenReader(path)
	}, nil
}

func OpenReader(path string) (io.ReadCloser, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("error opening fifo parent directory %q: %w", dir, err)
	}
	defer root.Close()

	// also uses O_NOFOLLOW under the hood
	f, err := root.OpenFile(base, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("error opening reader at %s: %w", path, err)
	}
	return f, nil
}

// OpenWriter opens a fifo file for writer, assuming it already exists, returns io.WriteCloser
func OpenWriter(path string) (io.WriteCloser, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("error opening fifo parent directory %q: %w", dir, err)
	}
	defer root.Close()

	// also uses O_NOFOLLOW under the hood
	f, err := root.OpenFile(base, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("error opening writer at %s: %w", path, err)
	}
	return f, nil
}

// Remove a fifo that already exists at a given path
func Remove(path string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("error opening root dir %q: %w", dir, err)
	}
	defer root.Close()

	return root.Remove(base)
}

func IsClosedErr(err error) bool {
	err2, ok := err.(*os.PathError)
	if ok {
		return err2.Err == os.ErrClosed
	}
	return false
}
