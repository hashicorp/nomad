// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// StreamingRPC may be satisfied by client.Client or server.Server.
type StreamingRPC interface {
	StreamingRpcHandler(method string) (structs.StreamingRpcHandler, error)
}

// StreamingRPCErrorTestCase is a test case to be passed to the
// assertStreamingRPCError func.
type StreamingRPCErrorTestCase struct {
	Name   string
	RPC    string
	Req    interface{}
	Assert func(error) bool
}

// AssertStreamingRPCError asserts a streaming RPC's error matches the given
// assertion in the test case.
func AssertStreamingRPCError(t *testing.T, s StreamingRPC, tc StreamingRPCErrorTestCase) {
	handler, err := s.StreamingRpcHandler(tc.RPC)
	require.NoError(t, err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error, 1)
	streamMsg := make(chan *cstructs.StreamErrWrapper, 1)

	// Start the handler
	go handler(p2)

	// Start the decoder
	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg cstructs.StreamErrWrapper
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "closed") {
					return
				}
				errCh <- fmt.Errorf("error decoding: %v", err)
			}

			streamMsg <- &msg
		}
	}()

	// Send the request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.NoError(t, encoder.Encode(tc.Req))

	timeout := time.After(5 * time.Second)

	for {
		select {
		case <-timeout:
			t.Fatal("timeout")
		case err := <-errCh:
			require.NoError(t, err)
		case msg := <-streamMsg:
			// Convert RpcError to error
			var err error
			if msg.Error != nil {
				err = msg.Error
			}
			require.True(t, tc.Assert(err), "(%T) %s", msg.Error, msg.Error)
			return
		}
	}
}
