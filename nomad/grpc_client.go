package nomad

import (
	"context"
	"net"

	"github.com/hashicorp/nomad/helper/pool"
	"google.golang.org/grpc"
)

type ClientConnPool struct {
}

func (c *ClientConnPool) ClientConn(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		addr,
		// use WithInsecure mode here because we handle the TLS wrapping in the
		// custom dialer based on logic around whether the server has TLS enabled.
		grpc.WithInsecure(),
		grpc.WithContextDialer(newDialer()),
		grpc.WithDisableRetry(),
		// nolint:staticcheck // there is no other supported alternative to WithBalancerName
		grpc.WithBalancerName("pick_first"))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// newDialer returns a gRPC dialer function that conditionally wraps the connection
// with TLS based on the Server.useTLS value.
func newDialer() func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}

		if err != nil {
			conn.Close()
			return nil, err
		}

		_, err = conn.Write([]byte{pool.RpcGRPC})
		if err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	}
}
