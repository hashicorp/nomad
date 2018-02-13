package structs

import (
	"fmt"
	"io"
	"sync"
)

// StreamingRpcHeader is the first struct serialized after entering the
// streaming RPC mode. The header is used to dispatch to the correct method.
type StreamingRpcHeader struct {
	// Method is the name of the method to invoke.
	Method string
}

// StreamingRpcAck is used to acknowledge receiving the StreamingRpcHeader and
// routing to the requirested handler.
type StreamingRpcAck struct {
	// Error is used to return whether an error occured establishing the
	// streaming RPC. This error occurs before entering the RPC handler.
	Error string
}

// StreamingRpcHandler defines the handler for a streaming RPC.
type StreamingRpcHandler func(conn io.ReadWriteCloser)

// StreamingRpcRegistery is used to add and retrieve handlers
type StreamingRpcRegistery struct {
	registry map[string]StreamingRpcHandler
}

// NewStreamingRpcRegistery creates a new registry. All registrations of
// handlers should be done before retrieving handlers.
func NewStreamingRpcRegistery() *StreamingRpcRegistery {
	return &StreamingRpcRegistery{
		registry: make(map[string]StreamingRpcHandler),
	}
}

// Register registers a new handler for the given method name
func (s *StreamingRpcRegistery) Register(method string, handler StreamingRpcHandler) {
	s.registry[method] = handler
}

// GetHandler returns a handler for the given method or an error if it doesn't exist.
func (s *StreamingRpcRegistery) GetHandler(method string) (StreamingRpcHandler, error) {
	h, ok := s.registry[method]
	if !ok {
		return nil, fmt.Errorf("%s: %q", ErrUnknownMethod, method)
	}

	return h, nil
}

// Bridge is used to just link two connections together and copy traffic
func Bridge(a, b io.ReadWriteCloser) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(a, b)
		a.Close()
		b.Close()
	}()
	go func() {
		defer wg.Done()
		io.Copy(b, a)
		a.Close()
		b.Close()
	}()
	wg.Wait()
}
