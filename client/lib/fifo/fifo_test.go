// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"bytes"
	"io"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFIFO tests basic behavior, and that reader closes when writer closes
func TestFIFO(t *testing.T) {
	require := require.New(t)
	var path string

	if runtime.GOOS == "windows" {
		path = "//./pipe/fifo"
	} else {
		path = filepath.Join(t.TempDir(), "fifo")
	}

	readerOpenFn, err := CreateAndRead(path)
	require.NoError(err)

	var reader io.ReadCloser

	toWrite := [][]byte{
		[]byte("abc\n"),
		[]byte(""),
		[]byte("def\n"),
		[]byte("nomad"),
		[]byte("\n"),
	}

	var readBuf bytes.Buffer
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()

		var err error
		reader, err = readerOpenFn()
		assert.NoError(t, err)
		if err != nil {
			return
		}

		_, err = io.Copy(&readBuf, reader)
		assert.NoError(t, err)
	}()

	writer, err := OpenWriter(path)
	require.NoError(err)
	for _, b := range toWrite {
		n, err := writer.Write(b)
		require.NoError(err)
		require.Equal(n, len(b))
	}
	require.NoError(writer.Close())
	time.Sleep(500 * time.Millisecond)

	wait.Wait()
	require.NoError(reader.Close())

	expected := "abc\ndef\nnomad\n"
	require.Equal(expected, readBuf.String())

	require.NoError(Remove(path))
}

// TestWriteClose asserts that when writer closes, subsequent Write() fails
func TestWriteClose(t *testing.T) {
	require := require.New(t)
	var path string

	if runtime.GOOS == "windows" {
		path = "//./pipe/" + uuid.Generate()[:4]
	} else {
		path = filepath.Join(t.TempDir(), "fifo")
	}

	readerOpenFn, err := CreateAndRead(path)
	require.NoError(err)
	var reader io.ReadCloser

	var readBuf bytes.Buffer
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()

		var err error
		reader, err = readerOpenFn()
		assert.NoError(t, err)
		if err != nil {
			return
		}

		_, err = io.Copy(&readBuf, reader)
		assert.NoError(t, err)
	}()

	writer, err := OpenWriter(path)
	require.NoError(err)

	var count int
	wait.Add(1)
	go func() {
		defer wait.Done()
		for count = 0; count < int(^uint16(0)); count++ {
			_, err := writer.Write([]byte(","))
			if err != nil && IsClosedErr(err) {
				break
			}
			require.NoError(err)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	require.NoError(writer.Close())
	wait.Wait()

	require.Equal(count, len(readBuf.String()))
}
