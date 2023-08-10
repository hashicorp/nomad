// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bufconndialer

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBufConnDialer asserts that bufconndialer creates a dialer connected to a
// listener.
func TestBufConnDialer(t *testing.T) {
	listener, dialer := New()

	cleanup := make(chan struct{})
	go func() {
		defer close(cleanup)
		for {
			conn, err := listener.Accept()
			if err != nil {
				// google.golang.org/grpc/test/bufconn.Listener doesn't
				// return a net.ErrClosed so we have to compare strings
				if err.Error() == "closed" {
					return
				}

				t.Errorf("error accepting connection: %v", err)
				return
			}

			n, err := conn.Write([]byte("ok"))
			if err != nil {
				t.Errorf("error writing to connection after %d bytes: %v", n, err)
				return
			}
			if err := conn.Close(); err != nil {
				t.Errorf("error closing connection: %v", err)
				return
			}
		}
	}()

	conn, err := dialer.Dial("anything", "goes")
	require.NoError(t, err)

	buf := make([]byte, 2)
	_, err = conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "ok", string(buf))

	_, err = conn.Read(buf)
	require.EqualError(t, err, io.EOF.Error())

	listener.Close()
	<-cleanup
}
