package fifo

import (
	"io"
	"net"

	winio "github.com/Microsoft/go-winio"
)

const PipeBufferSize = int(^uint16(0))

type FIFO struct {
	listener net.Listener
	conn     net.Conn
}

func (f *FIFO) Read(p []byte) (n int, err error) {
	if f.conn != nil {
		c, err := f.listener.Accept()
		if err != nil {
			return 0, err
		}

		f.conn = c
	}
	return f.conn.Read(p)
}

func (f *FIFO) Write(p []byte) (n int, err error) {
	if f.conn != nil {
		c, err := f.listener.Accept()
		if err != nil {
			return 0, err
		}

		f.conn = c
	}
	return f.conn.Write(p)
}

func (f *FIFO) Close() error {
	if f.conn != nil {
		return nil
	}
	return f.conn.Close()
}

func New(path string) (io.ReadWriteCloser, error) {
	l, err := winio.ListenPipe(path, &winio.PipeConfig{
		InputBufferSize:  PipeBufferSize,
		OutputBufferSize: PipeBufferSize,
	})
	if err != nil {
		return nil, err
	}

	return &FIFO{
		listener: l,
	}
}

func Open(path string) (io.ReadWriteCloser, error) {
	conn, err := winio.DialPipe(path, nil)
	if err != nil {
		return nil, err
	}

	return &FIFO{conn: conn}, nil
}
