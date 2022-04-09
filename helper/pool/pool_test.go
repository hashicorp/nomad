package pool

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/sdk/portfree"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *ConnPool {
	l := testlog.HCLogger(t)
	p := NewPool(l, 1*time.Minute, 10, nil)
	return p
}

func TestConnPool_ConnListener(t *testing.T) {
	port := portfree.New(t).GetOne()

	addrStr := fmt.Sprintf("127.0.0.1:%d", port)
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	require.Nil(t, err)

	exitCh := make(chan struct{})
	defer close(exitCh)
	go func() {
		ln, err := net.Listen("tcp", addrStr)
		require.Nil(t, err)
		defer ln.Close()
		conn, _ := ln.Accept()
		defer conn.Close()

		<-exitCh
	}()

	time.Sleep(100 * time.Millisecond)

	// Create a test pool
	pool := newTestPool(t)

	// Setup a listener
	c := make(chan *Conn, 1)
	pool.SetConnListener(c)

	// Make an RPC
	_, err = pool.acquire("test", addr)
	require.Nil(t, err)

	// Assert we get a connection.
	select {
	case <-c:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}

	// Test that the channel is closed when the pool shuts down.
	require.Nil(t, pool.Shutdown())
	_, ok := <-c
	require.False(t, ok)
}
