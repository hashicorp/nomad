// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/pprof"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitor_Monitor(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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

// Test that by default with no acl, endpoint is disabled
func TestAgentProfile_DefaultDisabled(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// start server and client
	s1, cleanup := nomad.TestServer(t, nil)
	defer cleanup()

	testutil.WaitForLeader(t, s1.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	req := structs.AgentPprofRequest{
		ReqType: pprof.CPUReq,
		NodeID:  c.NodeID(),
	}

	reply := structs.AgentPprofResponse{}

	err := c.ClientRPC("Agent.Profile", &req, &reply)
	require.EqualError(err, structs.ErrPermissionDenied.Error())
}

func TestAgentProfile(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// start server and client
	s1, cleanup := nomad.TestServer(t, nil)
	defer cleanup()

	testutil.WaitForLeader(t, s1.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.GetConfig().RPCAddr.String()}
		c.EnableDebug = true
	})
	defer cleanupC()

	// Successful request
	{
		req := structs.AgentPprofRequest{
			ReqType: pprof.CPUReq,
			NodeID:  c.NodeID(),
		}

		reply := structs.AgentPprofResponse{}

		err := c.ClientRPC("Agent.Profile", &req, &reply)
		require.NoError(err)

		require.NotNil(reply.Payload)
		require.Equal(c.NodeID(), reply.AgentID)
	}

	// Unknown profile request
	{
		req := structs.AgentPprofRequest{
			ReqType: pprof.LookupReq,
			Profile: "unknown",
			NodeID:  c.NodeID(),
		}

		reply := structs.AgentPprofResponse{}

		err := c.ClientRPC("Agent.Profile", &req, &reply)
		require.EqualError(err, "RPC Error:: 404,Pprof profile not found profile: unknown")
	}
}

func TestAgentProfile_ACL(t *testing.T) {
	ci.Parallel(t)

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

	policyBad := mock.AgentPolicy(acl.PolicyRead)
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.AgentPolicy(acl.PolicyWrite)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid", policyGood)

	cases := []struct {
		Name    string
		Token   string
		authErr bool
	}{
		{
			Name:    "bad token",
			Token:   tokenBad.SecretID,
			authErr: true,
		},
		{
			Name:  "good token",
			Token: tokenGood.SecretID,
		},
		{
			Name:  "root token",
			Token: root.SecretID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			req := &structs.AgentPprofRequest{
				ReqType: pprof.CmdReq,
				QueryOptions: structs.QueryOptions{
					Namespace: structs.DefaultNamespace,
					Region:    "global",
					AuthToken: tc.Token,
				},
			}

			reply := &structs.AgentPprofResponse{}

			err := c.ClientRPC("Agent.Profile", req, reply)
			if tc.authErr {
				require.EqualError(err, structs.ErrPermissionDenied.Error())
			} else {
				require.NoError(err)
				require.NotNil(reply.Payload)
			}
		})
	}
}

func TestAgentHost(t *testing.T) {
	ci.Parallel(t)

	// start server and client
	s1, cleanup := nomad.TestServer(t, nil)
	defer cleanup()

	testutil.WaitForLeader(t, s1.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.GetConfig().RPCAddr.String()}
		c.EnableDebug = true
	})
	defer cleanupC()

	req := structs.HostDataRequest{}
	var resp structs.HostDataResponse

	err := c.ClientRPC("Agent.Host", &req, &resp)
	require.NoError(t, err)

	require.NotNil(t, resp.HostData)
	require.Equal(t, c.NodeID(), resp.AgentID)
}

func TestAgentHost_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	policyGood := mock.AgentPolicy(acl.PolicyRead)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1005, "valid", policyGood)

	policyBad := mock.NodePolicy(acl.PolicyWrite)
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1009, "invalid", policyBad)

	cases := []struct {
		Name    string
		Token   string
		authErr bool
	}{
		{
			Name:    "bad token",
			Token:   tokenBad.SecretID,
			authErr: true,
		},
		{
			Name:  "good token",
			Token: tokenGood.SecretID,
		},
		{
			Name:  "root token",
			Token: root.SecretID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			req := structs.HostDataRequest{
				QueryOptions: structs.QueryOptions{
					AuthToken: tc.Token,
				},
			}
			var resp structs.HostDataResponse

			err := c.ClientRPC("Agent.Host", &req, &resp)
			if tc.authErr {
				require.EqualError(t, err, structs.ErrPermissionDenied.Error())
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, resp.HostData)
			}
		})
	}
}
