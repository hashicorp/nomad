// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package gatedwriter

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriter_impl(t *testing.T) {
	var _ io.Writer = new(Writer)
}

type slowTestWriter struct {
	buf       *bytes.Buffer
	called    chan struct{}
	callCount int
}

func (w *slowTestWriter) Write(p []byte) (int, error) {
	if w.callCount == 0 {
		defer close(w.called)
	}

	w.callCount++
	time.Sleep(time.Millisecond)

	return w.buf.Write(p)
}

func TestWriter_WithSlowWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	called := make(chan struct{})

	w := &slowTestWriter{
		buf:    buf,
		called: called,
	}

	writer := &Writer{Writer: w}

	writer.Write([]byte("foo\n"))
	writer.Write([]byte("bar\n"))
	writer.Write([]byte("baz\n"))

	flushed := make(chan struct{})

	go func() {
		writer.Flush()
		close(flushed)
	}()

	// wait for the flush to call Write on slowTestWriter
	<-called

	// write to the now-flushing writer, which is no longer buffering
	writer.Write([]byte("quux\n"))

	// wait for the flush to finish to assert
	<-flushed

	require.Equal(t, "foo\nbar\nbaz\nquux\n", buf.String())
}

func TestWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	w := &Writer{Writer: buf}
	w.Write([]byte("foo\n"))
	w.Write([]byte("bar\n"))

	if buf.String() != "" {
		t.Fatalf("bad: %s", buf.String())
	}

	w.Flush()

	if buf.String() != "foo\nbar\n" {
		t.Fatalf("bad: %s", buf.String())
	}

	w.Write([]byte("baz\n"))

	if buf.String() != "foo\nbar\nbaz\n" {
		t.Fatalf("bad: %s", buf.String())
	}
}

func TestWriter_WithMultipleWriters(t *testing.T) {
	buf := new(bytes.Buffer)

	writer := &Writer{Writer: buf}

	strs := []string{
		"foo\n",
		"bar\n",
		"baz\n",
		"quux\n",
	}

	waitCh := make(chan struct{})

	wg := &sync.WaitGroup{}

	for _, str := range strs {
		str := str
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-waitCh
			writer.Write([]byte(str))
		}()
	}

	// synchronize calls to Write() as closely as possible
	close(waitCh)

	wg.Wait()

	writer.Flush()

	require.Equal(t, strings.Count(buf.String(), "\n"), len(strs))
}
