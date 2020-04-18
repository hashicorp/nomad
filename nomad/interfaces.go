package nomad

import "github.com/hashicorp/nomad/nomad/state"

// RPCServer is a minimal interface of the Server, intended as
// an aid for testing logic surrounding server-to-server or
// server-to-client RPC calls
type RPCServer interface {
	RPC(method string, args interface{}, reply interface{}) error
	State() *state.StateStore
}
