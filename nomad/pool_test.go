package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *ConnPool {
	w := testlog.NewWriter(t)
	p := NewPool(w, 1*time.Minute, 10, nil)
	return p
}

func TestConnPool_ConnListener(t *testing.T) {
	// Create a server and test pool
	s := TestServer(t, nil)
	pool := newTestPool(t)

	// Setup a listener
	c := make(chan *yamux.Session, 1)
	pool.SetConnListener(c)

	// Make an RPC
	var out struct{}
	err := pool.RPC(s.Region(), s.config.RPCAddr, structs.ApiMajorVersion, "Status.Ping", struct{}{}, &out)
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
