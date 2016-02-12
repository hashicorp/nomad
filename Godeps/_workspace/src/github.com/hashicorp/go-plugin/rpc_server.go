package plugin

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"

	"github.com/hashicorp/yamux"
)

// RPCServer listens for network connections and then dispenses interface
// implementations over net/rpc.
type RPCServer struct {
	Plugins map[string]Plugin

	// Stdout, Stderr are what this server will use instead of the
	// normal stdin/out/err. This is because due to the multi-process nature
	// of our plugin system, we can't use the normal process values so we
	// make our own custom one we pipe across.
	Stdout io.Reader
	Stderr io.Reader
}

// Accept accepts connections on a listener and serves requests for
// each incoming connection. Accept blocks; the caller typically invokes
// it in a go statement.
func (s *RPCServer) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("[ERR] plugin server: %s", err)
			return
		}

		go s.ServeConn(conn)
	}
}

// ServeConn runs a single connection.
//
// ServeConn blocks, serving the connection until the client hangs up.
func (s *RPCServer) ServeConn(conn io.ReadWriteCloser) {
	// First create the yamux server to wrap this connection
	mux, err := yamux.Server(conn, nil)
	if err != nil {
		conn.Close()
		log.Printf("[ERR] plugin: %s", err)
		return
	}

	// Accept the control connection
	control, err := mux.Accept()
	if err != nil {
		mux.Close()
		if err != io.EOF {
			log.Printf("[ERR] plugin: %s", err)
		}

		return
	}

	// Connect the stdstreams (in, out, err)
	stdstream := make([]net.Conn, 2)
	for i, _ := range stdstream {
		stdstream[i], err = mux.Accept()
		if err != nil {
			mux.Close()
			log.Printf("[ERR] plugin: accepting stream %d: %s", i, err)
			return
		}
	}

	// Copy std streams out to the proper place
	go copyStream("stdout", stdstream[0], s.Stdout)
	go copyStream("stderr", stdstream[1], s.Stderr)

	// Create the broker and start it up
	broker := newMuxBroker(mux)
	go broker.Run()

	// Use the control connection to build the dispenser and serve the
	// connection.
	server := rpc.NewServer()
	server.RegisterName("Dispenser", &dispenseServer{
		broker:  broker,
		plugins: s.Plugins,
	})
	server.ServeConn(control)
}

// dispenseServer dispenses variousinterface implementations for Terraform.
type dispenseServer struct {
	broker  *MuxBroker
	plugins map[string]Plugin
}

func (d *dispenseServer) Dispense(
	name string, response *uint32) error {
	// Find the function to create this implementation
	p, ok := d.plugins[name]
	if !ok {
		return fmt.Errorf("unknown plugin type: %s", name)
	}

	// Create the implementation first so we know if there is an error.
	impl, err := p.Server(d.broker)
	if err != nil {
		// We turn the error into an errors error so that it works across RPC
		return errors.New(err.Error())
	}

	// Reserve an ID for our implementation
	id := d.broker.NextId()
	*response = id

	// Run the rest in a goroutine since it can only happen once this RPC
	// call returns. We wait for a connection for the plugin implementation
	// and serve it.
	go func() {
		conn, err := d.broker.Accept(id)
		if err != nil {
			log.Printf("[ERR] Plugin dispense %s: %s", name, err)
			return
		}

		serve(conn, "Plugin", impl)
	}()

	return nil
}

func serve(conn io.ReadWriteCloser, name string, v interface{}) {
	server := rpc.NewServer()
	if err := server.RegisterName(name, v); err != nil {
		log.Printf("[ERR] Plugin dispense: %s", err)
		return
	}

	server.ServeConn(conn)
}
