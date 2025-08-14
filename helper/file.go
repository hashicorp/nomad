// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"io"
	"os"
)

// ReadFileContent is a helper that mimics the stdlib ReadFile implementation,
// but accepts an already opened file handle. This is useful when using os.Root
// functionality such as OpenInRoot which does not have convenient read methods.
func ReadFileContent(file *os.File) ([]byte, error) {

	var size int
	if info, err := file.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}
	size++ // one byte for final read at EOF

	// If a file claims a small size, read at least 512 bytes. In particular,
	// files in Linux's /proc claim size 0 but then do not work right if read in
	// small pieces, so an initial read of 1 byte would not work correctly.
	if size < 512 {
		size = 512
	}

	data := make([]byte, 0, size)
	for {
		n, err := file.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}

		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0) //nolint:gocritic
			data = d[:len(data)]
		}
	}
}
