package client

import (
	"errors"
	"io"
	"net"
	"net/rpc"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/client/servers"
	inmem "github.com/hashicorp/nomad/helper/codec"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/nomad/structs"
)

// rpcEndpoints holds the RPC endpoints
type rpcEndpoints struct {
	ClientStats *ClientStats
	CSI         *CSI
	FileSystem  *FileSystem
	Allocations *Allocations
	Agent       *Agent
}

// ClientRPC is used to make a local, client only RPC call
func (c *Client) ClientRPC(method string, args interface{}, reply interface{}) error {
	codec := &inmem.InmemCodec{
		Method: method,
		Args:   args,
		Reply:  reply,
	}
	if err := c.rpcServer.ServeRequest(codec); err != nil {
		return err
	}
	return codec.Err
}

// StreamingRpcHandler is used to make a local, client only streaming RPC
// call.
func (c *Client) StreamingRpcHandler(method string) (structs.StreamingRpcHandler, error) {
	return c.streamingRpcs.GetHandler(method)
}

// RPC is used to forward an RPC call to a nomad server, or fail if no servers.
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// Invoke the RPCHandler if it exists
	if c.config.RPCHandler != nil {
		return c.config.RPCHandler.RPC(method, args, reply)
	}

	// We will try to automatically retry requests that fail due to things like server unavailability
	// but instead of retrying forever, lets have a solid upper-bound
	deadline := time.Now()

	// A reasonable amount of time for leader election. Note when servers forward() our RPC requests
	// to the leader they may also allow for an RPCHoldTimeout while waiting for leader election.
	// That's OK, we won't double up because we are using it here not as a sleep but
	// as a hint to give up
	deadline = deadline.Add(c.config.RPCHoldTimeout)

	// If its a blocking query, allow the time specified by the request
	if info, ok := args.(structs.RPCInfo); ok {
		deadline = deadline.Add(info.TimeToBlock())
	}

TRY:
	server := c.servers.FindServer()
	if server == nil {
		return noServersErr
	}

	// Make the request.
	rpcErr := c.connPool.RPC(c.Region(), server.Addr, method, args, reply)

	if rpcErr == nil {
		c.fireRpcRetryWatcher()
		return nil
	}

	// If shutting down, exit without logging the error
	select {
	case <-c.shutdownCh:
		return nil
	default:
	}

	// Move off to another server, and see if we can retry.
	c.rpcLogger.Error("error performing RPC to server", "error", rpcErr, "rpc", method, "server", server.Addr)
	c.servers.NotifyFailedServer(server)

	if !canRetry(args, rpcErr) {
		c.rpcLogger.Error("error performing RPC to server which is not safe to automatically retry", "error", rpcErr, "rpc", method, "server", server.Addr)
		return rpcErr
	}
	if time.Now().After(deadline) {
		// Blocking queries are tricky.  jitters and rpcholdtimes in multiple places can result in our server call taking longer than we wanted it to. For example:
		// a block time of 5s may easily turn into the server blocking for 10s since it applies its own RPCHoldTime. If the server dies at t=7s we still want to retry
		// so before we give up on blocking queries make one last attempt for an immediate answer
		if info, ok := args.(structs.RPCInfo); ok && info.TimeToBlock() > 0 {
			info.SetTimeToBlock(0)
			return c.RPC(method, args, reply)
		}
		c.rpcLogger.Error("error performing RPC to server, deadline exceeded, cannot retry", "error", rpcErr, "rpc", method, "server", server.Addr)
		return rpcErr
	}

	// Wait to avoid thundering herd
	select {
	case <-time.After(lib.RandomStagger(c.config.RPCHoldTimeout / structs.JitterFraction)):
		// If we are going to retry a blocking query we need to update the time to block so it finishes by our deadline.
		if info, ok := args.(structs.RPCInfo); ok && info.TimeToBlock() > 0 {
			newBlockTime := time.Until(deadline)
			// We can get below 0 here on slow computers because we slept for jitter so at least try to get an immediate response
			if newBlockTime < 0 {
				newBlockTime = 0
			}
			info.SetTimeToBlock(newBlockTime)
			return c.RPC(method, args, reply)
		}

		goto TRY
	case <-c.shutdownCh:
	}
	return rpcErr
}

// canRetry returns true if the given situation is safe for a retry.
func canRetry(args interface{}, err error) bool {
	// No leader errors are always safe to retry since no state could have
	// been changed.
	if structs.IsErrNoLeader(err) {
		return true
	}

	// Reads are safe to retry for stream errors, such as if a server was
	// being shut down.
	info, ok := args.(structs.RPCInfo)
	if ok && info.IsRead() && lib.IsErrEOF(err) {
		return true
	}

	return false
}

// RemoteStreamingRpcHandler is used to make a streaming RPC call to a remote
// server.
func (c *Client) RemoteStreamingRpcHandler(method string) (structs.StreamingRpcHandler, error) {
	server := c.servers.FindServer()
	if server == nil {
		return nil, noServersErr
	}

	conn, err := c.streamingRpcConn(server, method)
	if err != nil {
		// Move off to another server
		c.rpcLogger.Error("error performing RPC to server", "error", err, "rpc", method, "server", server.Addr)
		c.servers.NotifyFailedServer(server)
		return nil, err
	}

	return bridgedStreamingRpcHandler(conn), nil
}

// bridgedStreamingRpcHandler creates a bridged streaming RPC handler by copying
// data between the two sides.
func bridgedStreamingRpcHandler(sideA io.ReadWriteCloser) structs.StreamingRpcHandler {
	return func(sideB io.ReadWriteCloser) {
		defer sideA.Close()
		defer sideB.Close()
		structs.Bridge(sideA, sideB)
	}
}

// streamingRpcConn is used to retrieve a connection to a server to conduct a
// streaming RPC.
func (c *Client) streamingRpcConn(server *servers.Server, method string) (net.Conn, error) {
	// Dial the server
	conn, err := net.DialTimeout("tcp", server.Addr.String(), 10*time.Second)
	if err != nil {
		return nil, err
	}

	// Cast to TCPConn
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetNoDelay(true)
	}

	// Check if TLS is enabled
	c.tlsWrapLock.RLock()
	tlsWrap := c.tlsWrap
	c.tlsWrapLock.RUnlock()

	if tlsWrap != nil {
		// Switch the connection into TLS mode
		if _, err := conn.Write([]byte{byte(pool.RpcTLS)}); err != nil {
			conn.Close()
			return nil, err
		}

		// Wrap the connection in a TLS client
		tlsConn, err := tlsWrap(c.Region(), conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}

	// Write the multiplex byte to set the mode
	if _, err := conn.Write([]byte{byte(pool.RpcStreaming)}); err != nil {
		conn.Close()
		return nil, err
	}

	// Send the header
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	header := structs.StreamingRpcHeader{
		Method: method,
	}
	if err := encoder.Encode(header); err != nil {
		conn.Close()
		return nil, err
	}

	// Wait for the acknowledgement
	var ack structs.StreamingRpcAck
	if err := decoder.Decode(&ack); err != nil {
		conn.Close()
		return nil, err
	}

	if ack.Error != "" {
		conn.Close()
		return nil, errors.New(ack.Error)
	}

	return conn, nil
}

// setupClientRpc is used to setup the Client's RPC endpoints
func (c *Client) setupClientRpc(rpcs map[string]interface{}) {
	// Create the RPC Server
	c.rpcServer = rpc.NewServer()

	// Initialize the RPC handlers
	if rpcs != nil {
		// override RPCs
		for name, rpc := range rpcs {
			c.rpcServer.RegisterName(name, rpc)
		}
	} else {
		c.endpoints.ClientStats = &ClientStats{c}
		c.endpoints.CSI = &CSI{c}
		c.endpoints.FileSystem = NewFileSystemEndpoint(c)
		c.endpoints.Allocations = NewAllocationsEndpoint(c)
		c.endpoints.Agent = NewAgentEndpoint(c)
		c.setupClientRpcServer(c.rpcServer)
	}

	go c.rpcConnListener()
}

// setupClientRpcServer is used to populate a client RPC server with endpoints.
func (c *Client) setupClientRpcServer(server *rpc.Server) {
	// Register the endpoints
	server.Register(c.endpoints.ClientStats)
	server.Register(c.endpoints.CSI)
	server.Register(c.endpoints.FileSystem)
	server.Register(c.endpoints.Allocations)
	server.Register(c.endpoints.Agent)
}

// rpcConnListener is a long lived function that listens for new connections
// being made on the connection pool and starts an RPC listener for each
// connection.
func (c *Client) rpcConnListener() {
	// Make a channel for new connections.
	conns := make(chan *pool.Conn, 4)
	c.connPool.SetConnListener(conns)

	for {
		select {
		case <-c.shutdownCh:
			return
		case conn, ok := <-conns:
			if !ok {
				continue
			}

			go c.listenConn(conn)
		}
	}
}

// listenConn is used to listen for connections being made from the server on
// pre-existing connection. This should be called in a goroutine.
func (c *Client) listenConn(conn *pool.Conn) {
	for {
		stream, err := conn.AcceptStream()
		if err != nil {
			if conn.IsClosed() {
				return
			}

			c.rpcLogger.Error("failed to accept RPC conn", "error", err)
			continue
		}

		go c.handleConn(stream)
		metrics.IncrCounter([]string{"client", "rpc", "accept_conn"}, 1)
	}
}

// handleConn is used to determine if this is a RPC or Streaming RPC connection and
// invoke the correct handler
func (c *Client) handleConn(conn net.Conn) {
	// Read a single byte
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			c.rpcLogger.Error("error reading byte", "error", err)
		}
		conn.Close()
		return
	}

	// Switch on the byte
	switch pool.RPCType(buf[0]) {
	case pool.RpcNomad:
		c.handleNomadConn(conn)

	case pool.RpcStreaming:
		c.handleStreamingConn(conn)

	default:
		c.rpcLogger.Error("unrecognized RPC byte", "byte", buf[0])
		conn.Close()
		return
	}
}

// handleNomadConn is used to handle a single Nomad RPC connection.
func (c *Client) handleNomadConn(conn net.Conn) {
	defer conn.Close()
	rpcCodec := pool.NewServerCodec(conn)
	for {
		select {
		case <-c.shutdownCh:
			return
		default:
		}

		if err := c.rpcServer.ServeRequest(rpcCodec); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				c.rpcLogger.Error("error performing RPC", "error", err, "addr", conn.RemoteAddr())
				metrics.IncrCounter([]string{"client", "rpc", "request_error"}, 1)
			}
			return
		}
		metrics.IncrCounter([]string{"client", "rpc", "request"}, 1)
	}
}

// handleStreamingConn is used to handle a single Streaming Nomad RPC connection.
func (c *Client) handleStreamingConn(conn net.Conn) {
	defer conn.Close()

	// Decode the header
	var header structs.StreamingRpcHeader
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	if err := decoder.Decode(&header); err != nil {
		if err != io.EOF && !strings.Contains(err.Error(), "closed") {
			c.rpcLogger.Error("error performing streaming RPC", "error", err, "addr", conn.RemoteAddr())
			metrics.IncrCounter([]string{"client", "streaming_rpc", "request_error"}, 1)
		}

		return
	}

	ack := structs.StreamingRpcAck{}
	handler, err := c.streamingRpcs.GetHandler(header.Method)
	if err != nil {
		c.rpcLogger.Error("streaming RPC error", "addr", conn.RemoteAddr(), "error", err)
		metrics.IncrCounter([]string{"client", "streaming_rpc", "request_error"}, 1)
		ack.Error = err.Error()
	}

	// Send the acknowledgement
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)
	if err := encoder.Encode(ack); err != nil {
		conn.Close()
		return
	}

	if ack.Error != "" {
		return
	}

	// Invoke the handler
	metrics.IncrCounter([]string{"client", "streaming_rpc", "request"}, 1)
	handler(conn)
}

// resolveServer given a sever's address as a string, return it's resolved
// net.Addr or an error.
func resolveServer(s string) (net.Addr, error) {
	const defaultClientPort = "4647" // default client RPC port
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		if strings.Contains(err.Error(), "missing port") {
			host = s
			port = defaultClientPort
		} else {
			return nil, err
		}
	}
	return net.ResolveTCPAddr("tcp", net.JoinHostPort(host, port))
}

// Ping is used to ping a particular server and returns whether it is healthy or
// a potential error.
func (c *Client) Ping(srv net.Addr) error {
	var reply struct{}
	err := c.connPool.RPC(c.Region(), srv, "Status.Ping", struct{}{}, &reply)
	return err
}

// rpcRetryWatcher returns a channel that will be closed if an event happens
// such that we expect the next RPC to be successful.
func (c *Client) rpcRetryWatcher() <-chan struct{} {
	c.rpcRetryLock.Lock()
	defer c.rpcRetryLock.Unlock()

	if c.rpcRetryCh == nil {
		c.rpcRetryCh = make(chan struct{})
	}

	return c.rpcRetryCh
}

// fireRpcRetryWatcher causes any RPC retryloops to retry their RPCs because we
// believe the will be successful.
func (c *Client) fireRpcRetryWatcher() {
	c.rpcRetryLock.Lock()
	defer c.rpcRetryLock.Unlock()
	if c.rpcRetryCh != nil {
		close(c.rpcRetryCh)
		c.rpcRetryCh = nil
	}
}
