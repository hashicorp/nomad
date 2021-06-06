package pool

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/freeport"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *ConnPool {
	l := testlog.HCLogger(t)
	p := NewPool(l, 1*time.Minute, 10, nil)
	return p
}

func TestConnPool_ConnListener(t *testing.T) {
	require := require.New(t)

	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

	addrStr := fmt.Sprintf("127.0.0.1:%d", ports[0])
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	require.Nil(err)

	exitCh := make(chan struct{})
	defer close(exitCh)
	go func() {
		ln, err := net.Listen("tcp", addrStr)
		require.Nil(err)
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
	_, err = pool.acquire("test", addr, structs.ApiMajorVersion)
	require.Nil(err)

	// Assert we get a connection.
	select {
	case <-c:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}

	// Test that the channel is closed when the pool shuts down.
	require.Nil(pool.Shutdown())
	_, ok := <-c
	require.False(ok)
}
