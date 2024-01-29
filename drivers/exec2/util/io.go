// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package util

import (
	"io"
)

// NullCloser returns an implementation of io.WriteCloser that will always
// succeed on the call to Close().
func NullCloser(w io.Writer) io.WriteCloser {
	return &writeCloser{w}
}

type writeCloser struct {
	io.Writer
}

func (*writeCloser) Close() error {
	return nil
}
