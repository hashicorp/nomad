// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// PipeBufferSize is the size of the input and output buffers for the windows
// named pipe
const PipeBufferSize = int32(^uint16(0))

type winFIFO struct {
	listener net.Listener
	conn     net.Conn
	connLock sync.Mutex
}

func (f *winFIFO) ensureConn() (net.Conn, error) {
	f.connLock.Lock()
	defer f.connLock.Unlock()
	if f.conn == nil {
		c, err := f.listener.Accept()
		if err != nil {
			return nil, err
		}
		f.conn = c
	}

	return f.conn, nil
}

func (f *winFIFO) Read(p []byte) (n int, err error) {
	conn, err := f.ensureConn()
	if err != nil {
		return 0, err
	}

	// If the connection is closed then we need to close the listener
	// to emulate unix fifo behavior
	n, err = conn.Read(p)
	if err == io.EOF {
		f.listener.Close()
	}
	return n, err
}

func (f *winFIFO) Write(p []byte) (n int, err error) {
	conn, err := f.ensureConn()
	if err != nil {
		return 0, err
	}

	// If the connection is closed then we need to close the listener
	// to emulate unix fifo behavior
	n, err = conn.Write(p)
	if err == io.EOF {
		conn.Close()
		f.listener.Close()
	}
	return n, err

}

func (f *winFIFO) Close() error {
	f.connLock.Lock()
	if f.conn != nil {
		f.conn.Close()
	}
	f.connLock.Unlock()
	return f.listener.Close()
}

// CreateAndRead creates a fifo at the given path and returns an io.ReadCloser open for it.
// The fifo must not already exist
func CreateAndRead(path string) (func() (io.ReadCloser, error), error) {
	l, err := winio.ListenPipe(path, &winio.PipeConfig{
		InputBufferSize:  PipeBufferSize,
		OutputBufferSize: PipeBufferSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create fifo: %v", err)
	}

	return func() (io.ReadCloser, error) {
		return &winFIFO{
			listener: l,
		}, nil
	}, nil
}

func OpenReader(path string) (io.ReadCloser, error) {
	l, err := winio.ListenOnlyPipe(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open fifo listener: %v", err)
	}

	return &winFIFO{listener: l}, nil
}

// OpenWriter opens a fifo that already exists and returns an io.WriteCloser for it
func OpenWriter(path string) (io.WriteCloser, error) {
	return winio.DialPipe(path, nil)
}

// Remove a fifo that already exists at a given path
func Remove(path string) error {
	dur := 500 * time.Millisecond
	conn, err := winio.DialPipe(path, &dur)
	if err == nil {
		return conn.Close()
	}

	os.Remove(path)
	return nil
}

func IsClosedErr(err error) bool {
	return err == winio.ErrFileClosed
}
