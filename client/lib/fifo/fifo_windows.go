package fifo

import (
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

func (f *winFIFO) Read(p []byte) (n int, err error) {
	f.connLock.Lock()
	defer f.connLock.Unlock()
	if f.conn == nil {
		c, err := f.listener.Accept()
		if err != nil {
			return 0, err
		}

		f.conn = c
	}

	// If the connection is closed then we need to close the listener
	// to emulate unix fifo behavior
	n, err = f.conn.Read(p)
	if err == io.EOF {
		f.listener.Close()
	}
	return n, err
}

func (f *winFIFO) Write(p []byte) (n int, err error) {
	f.connLock.Lock()
	defer f.connLock.Unlock()
	if f.conn == nil {
		c, err := f.listener.Accept()
		if err != nil {
			return 0, err
		}

		f.conn = c
	}

	// If the connection is closed then we need to close the listener
	// to emulate unix fifo behavior
	n, err = f.conn.Write(p)
	if err == io.EOF {
		f.listener.Close()
	}
	return n, err

}

func (f *winFIFO) Close() error {
	return f.listener.Close()
}

// New creates a fifo at the given path and returns an io.ReadWriteCloser for it. The fifo
// must not already exist
func New(path string) (io.ReadWriteCloser, error) {
	l, err := winio.ListenPipe(path, &winio.PipeConfig{
		InputBufferSize:  PipeBufferSize,
		OutputBufferSize: PipeBufferSize,
	})
	if err != nil {
		return nil, err
	}

	return &winFIFO{
		listener: l,
	}, nil
}

// OpenWriter opens a fifo that already exists and returns an io.ReadWriteCloser for it
func Open(path string) (io.ReadWriteCloser, error) {
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
