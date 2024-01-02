// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows
// +build !windows

package fifo

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// CreateAndRead creates a fifo at the given path, and returns an open function for reading.
// For compatibility with windows, the fifo must not exist already.
//
// It returns a reader open function that may block until a writer opens
// so it's advised to run it in a goroutine different from reader goroutine
func CreateAndRead(path string) (func() (io.ReadCloser, error), error) {
	// create first
	if err := mkfifo(path, 0600); err != nil {
		return nil, fmt.Errorf("error creating fifo %v: %v", path, err)
	}

	return func() (io.ReadCloser, error) {
		return OpenReader(path)
	}, nil
}

func OpenReader(path string) (io.ReadCloser, error) {
	return os.OpenFile(path, unix.O_RDONLY, os.ModeNamedPipe)
}

// OpenWriter opens a fifo file for writer, assuming it already exists, returns io.WriteCloser
func OpenWriter(path string) (io.WriteCloser, error) {
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
