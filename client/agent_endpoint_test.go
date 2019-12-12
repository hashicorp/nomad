package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client/config"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
)

func TestMonitor_Monitor(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// start server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	req := cstructs.MonitorRequest{
		LogLevel: "debug",
		NodeID:   c.NodeID(),
	}

	handler, err := c.StreamingRpcHandler("Agent.Monitor")
	require.Nil(err)

	// create pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	go handler(p2)

	// Start decoder
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

	// send request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(encoder.Encode(req))

	timeout := time.After(5 * time.Second)
	expected := "[DEBUG]"
	received := ""

OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for logs")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			if msg.Error != nil {
				t.Fatalf("Got error: %v", msg.Error.Error())
			}

			var frame sframer.StreamFrame
			err := json.Unmarshal(msg.Payload, &frame)
			assert.NoError(t, err)

			received += string(frame.Data)
			if strings.Contains(received, expected) {
				require.Nil(p2.Close())
				break OUTER
			}
		}
	}
}

func TestMonitor_Monitor_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// start server
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	policyBad := mock.NodePolicy(acl.PolicyDeny)
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.AgentPolicy(acl.PolicyRead)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid", policyGood)

	cases := []struct {
		Name        string
		Token       string
		ExpectedErr string
	}{
		{
			Name:        "bad token",
			Token:       tokenBad.SecretID,
			ExpectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:        "good token",
			Token:       tokenGood.SecretID,
			ExpectedErr: "Unknown log level",
		},
		{
			Name:        "root token",
			Token:       root.SecretID,
			ExpectedErr: "Unknown log level",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			req := &cstructs.MonitorRequest{
				LogLevel: "unknown",
				QueryOptions: structs.QueryOptions{
					Namespace: structs.DefaultNamespace,
					Region:    "global",
					AuthToken: tc.Token,
				},
			}

			handler, err := c.StreamingRpcHandler("Agent.Monitor")
			require.Nil(err)

			// create pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			streamMsg := make(chan *cstructs.StreamErrWrapper)

			go handler(p2)

			// Start decoder
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

			// send request
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			require.Nil(encoder.Encode(req))

			timeout := time.After(5 * time.Second)
		OUTER:
			for {
				select {
				case <-timeout:
					t.Fatal("timeout")
				case err := <-errCh:
					t.Fatal(err)
				case msg := <-streamMsg:
					if msg.Error == nil {
						continue
					}

					if strings.Contains(msg.Error.Error(), tc.ExpectedErr) {
						break OUTER
					} else {
						t.Fatalf("Bad error: %v", msg.Error)
					}
				}
			}
		})
	}
}
