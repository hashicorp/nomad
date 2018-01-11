package client

import (
	"fmt"
	"net"
	"net/rpc"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/codec"
)

// rpcEndpoints holds the RPC endpoints
type rpcEndpoints struct {
	ClientStats *ClientStats
}

// ClientRPC is used to make a local, client only RPC call
func (c *Client) ClientRPC(method string, args interface{}, reply interface{}) error {
	codec := &codec.InmemCodec{
		Method: method,
		Args:   args,
		Reply:  reply,
	}
	if err := c.rpcServer.ServeRequest(codec); err != nil {
		return err
	}
	return codec.Err
}

// RPC is used to forward an RPC call to a nomad server, or fail if no servers.
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// Invoke the RPCHandler if it exists
	if c.config.RPCHandler != nil {
		return c.config.RPCHandler.RPC(method, args, reply)
	}

	servers := c.servers.all()
	if len(servers) == 0 {
		return noServersErr
	}

	var mErr multierror.Error
	for _, s := range servers {
		// Make the RPC request
		if err := c.connPool.RPC(c.Region(), s.addr, c.RPCMajorVersion(), method, args, reply); err != nil {
			errmsg := fmt.Errorf("RPC failed to server %s: %v", s.addr, err)
			mErr.Errors = append(mErr.Errors, errmsg)
			c.logger.Printf("[DEBUG] client: %v", errmsg)
			c.servers.failed(s)
			continue
		}
		c.servers.good(s)
		return nil
	}

	return mErr.ErrorOrNil()
}

// setupClientRpc is used to setup the Client's RPC endpoints
func (c *Client) setupClientRpc() {
	// Initialize the RPC handlers
	c.endpoints.ClientStats = &ClientStats{c}

	// Create the RPC Server
	c.rpcServer = rpc.NewServer()

	// Register the endpoints with the RPC server
	c.setupClientRpcServer(c.rpcServer)
}

// setupClientRpcServer is used to populate a client RPC server with endpoints.
func (c *Client) setupClientRpcServer(server *rpc.Server) {
	// Register the endpoints
	server.Register(c.endpoints.ClientStats)
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
