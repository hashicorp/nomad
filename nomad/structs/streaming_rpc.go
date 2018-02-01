package structs

import (
	"errors"
	"io"
	"strings"
	"sync"
)

// TODO(alexdadgar): move to errors.go
const (
	errUnknownMethod = "unknown rpc method"
)

var (
	// ErrUnknownMethod is used to indicate that the requested method
	// is unknown.
	ErrUnknownMethod = errors.New(errUnknownMethod)
)

// IsErrUnknownMethod returns whether the error is due to the operation not
// being allowed due to lack of permissions.
func IsErrUnknownMethod(err error) bool {
	return err != nil && strings.Contains(err.Error(), errUnknownMethod)
}

// StreamingRpcHeader is the first struct serialized after entering the
// streaming RPC mode. The header is used to dispatch to the correct method.
type StreamingRpcHeader struct {
	// Method is the name of the method to invoke.
	Method string

	// QueryOptions and WriteRequest provide metadata about the RPC request.
	QueryOptions *QueryOptions
	WriteRequest *WriteRequest
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
		return nil, ErrUnknownMethod
	}

	return h, nil
}

// Bridge is used to just link two connections together and copy traffic
func Bridge(a, b io.ReadWriteCloser) error {
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
	return nil
}
