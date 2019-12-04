package nomad

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
)

func TestMonitor_Monitor_Remote_Client(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// start server and client
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// No node ID to monitor the remote server
	req := cstructs.MonitorRequest{
		LogLevel: "debug",
		NodeID:   c.NodeID(),
	}

	handler, err := s1.StreamingRpcHandler("Agent.Monitor")
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

	timeout := time.After(3 * time.Second)
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

func TestMonitor_Monitor_RemoteServer(t *testing.T) {
	t.Parallel()

	// start servers
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// determine leader and nonleader
	servers := []*Server{s1, s2}
	var nonLeader *Server
	var leader *Server
	for _, s := range servers {
		if !s.IsLeader() {
			nonLeader = s
		} else {
			leader = s
		}
	}

	cases := []struct {
		desc        string
		serverID    string
		expectedLog string
		logger      hclog.InterceptLogger
		origin      *Server
	}{
		{
			desc:        "remote leader",
			serverID:    "leader",
			expectedLog: "leader log",
			logger:      leader.logger,
			origin:      nonLeader,
		},
		{
			desc:        "remote server",
			serverID:    nonLeader.serf.LocalMember().Name,
			expectedLog: "nonleader log",
			logger:      nonLeader.logger,
			origin:      leader,
		},
		{
			desc:        "serverID is current leader",
			serverID:    "leader",
			expectedLog: "leader log",
			logger:      leader.logger,
			origin:      leader,
		},
		{
			desc:        "serverID is current server",
			serverID:    nonLeader.serf.LocalMember().Name,
			expectedLog: "non leader log",
			logger:      nonLeader.logger,
			origin:      nonLeader,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)

			// send some specific logs
			doneCh := make(chan struct{})
			go func() {
				for {
					select {
					case <-doneCh:
						return
					default:
						tc.logger.Warn(tc.expectedLog)
						time.Sleep(10 * time.Millisecond)
					}
				}
			}()

			req := cstructs.MonitorRequest{
				LogLevel: "warn",
				ServerID: tc.serverID,
			}

			handler, err := tc.origin.StreamingRpcHandler("Agent.Monitor")
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

			timeout := time.After(2 * time.Second)
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
					if strings.Contains(received, tc.expectedLog) {
						close(doneCh)
						require.Nil(p2.Close())
						break OUTER
					}
				}
			}
		})
	}
}

func TestMonitor_MonitorServer(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// start server
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	// No node ID to monitor the remote server
	req := cstructs.MonitorRequest{
		LogLevel: "debug",
	}

	handler, err := s.StreamingRpcHandler("Agent.Monitor")
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

	timeout := time.After(1 * time.Second)
	expected := "[DEBUG]"
	received := ""

	// send logs
	go func() {
		for {
			s.logger.Debug("test log")
			time.Sleep(100 * time.Millisecond)
		}
	}()

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
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
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

			handler, err := s.StreamingRpcHandler("Agent.Monitor")
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
