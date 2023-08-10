// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_AgentSelf(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/self", nil)
		require.NoError(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AgentSelfRequest(respW, req)
		require.NoError(err)

		// Check the job
		self := obj.(agentSelf)
		require.NotNil(self.Config)
		require.NotNil(self.Config.ACL)
		require.NotEmpty(self.Stats)

		// Check the Vault config
		require.Empty(self.Config.Vault.Token)

		// Assign a Vault token and require it is redacted.
		s.Config.Vault.Token = "badc0deb-adc0-deba-dc0d-ebadc0debadc"
		respW = httptest.NewRecorder()
		obj, err = s.Server.AgentSelfRequest(respW, req)
		require.NoError(err)
		self = obj.(agentSelf)
		require.Equal("<redacted>", self.Config.Vault.Token)

		// Assign a ReplicationToken token and require it is redacted.
		s.Config.ACL.ReplicationToken = "badc0deb-adc0-deba-dc0d-ebadc0debadc"
		respW = httptest.NewRecorder()
		obj, err = s.Server.AgentSelfRequest(respW, req)
		require.NoError(err)
		self = obj.(agentSelf)
		require.Equal("<redacted>", self.Config.ACL.ReplicationToken)

		// Check the Consul config
		require.Empty(self.Config.Consul.Token)

		// Assign a Consul token and require it is redacted.
		s.Config.Consul.Token = "badc0deb-adc0-deba-dc0d-ebadc0debadc"
		respW = httptest.NewRecorder()
		obj, err = s.Server.AgentSelfRequest(respW, req)
		require.NoError(err)
		self = obj.(agentSelf)
		require.Equal("<redacted>", self.Config.Consul.Token)

		// Check the Circonus config
		require.Empty(self.Config.Telemetry.CirconusAPIToken)

		// Assign a Consul token and require it is redacted.
		s.Config.Telemetry.CirconusAPIToken = "badc0deb-adc0-deba-dc0d-ebadc0debadc"
		respW = httptest.NewRecorder()
		obj, err = s.Server.AgentSelfRequest(respW, req)
		require.NoError(err)
		self = obj.(agentSelf)
		require.Equal("<redacted>", self.Config.Telemetry.CirconusAPIToken)
	})
}

func TestHTTP_AgentSelf_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/self", nil)
		require.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.AgentSelfRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err := s.Server.AgentSelfRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.AgentPolicy(acl.PolicyWrite))
			setToken(req, token)
			obj, err := s.Server.AgentSelfRequest(respW, req)
			require.Nil(err)

			self := obj.(agentSelf)
			require.NotNil(self.Config)
			require.NotNil(self.Stats)
		}

		// Try request with a root token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			obj, err := s.Server.AgentSelfRequest(respW, req)
			require.Nil(err)

			self := obj.(agentSelf)
			require.NotNil(self.Config)
			require.NotNil(self.Stats)
		}
	})
}

func TestHTTP_AgentJoin(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Determine the join address
		member := s.Agent.Server().LocalMember()
		addr := net.JoinHostPort(member.Addr.String(), strconv.Itoa(int(member.Port)))

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut,
			fmt.Sprintf("/v1/agent/join?address=%s&address=%s", addr, addr), nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AgentJoinRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the job
		join := obj.(joinResult)
		if join.NumJoined != 2 {
			t.Fatalf("bad: %#v", join)
		}
		if join.Error != "" {
			t.Fatalf("bad: %#v", join)
		}
	})
}

func TestHTTP_AgentMembers(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/members", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AgentMembersRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the job
		members := obj.(structs.ServerMembersResponse)
		if len(members.Members) != 1 {
			t.Fatalf("bad: %#v", members.Members)
		}
	})
}

func TestHTTP_AgentMembers_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/members", nil)
		require.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.AgentMembersRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.AgentPolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err := s.Server.AgentMembersRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			obj, err := s.Server.AgentMembersRequest(respW, req)
			require.Nil(err)

			members := obj.(structs.ServerMembersResponse)
			require.Len(members.Members, 1)
		}

		// Try request with a root token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			obj, err := s.Server.AgentMembersRequest(respW, req)
			require.Nil(err)

			members := obj.(structs.ServerMembersResponse)
			require.Len(members.Members, 1)
		}
	})
}

func TestHTTP_AgentMonitor(t *testing.T) {
	ci.Parallel(t)

	t.Run("invalid log_json parameter", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/monitor?log_json=no", nil)
			require.NoError(t, err)
			resp := newClosableRecorder()

			// Make the request
			_, err = s.Server.AgentMonitor(resp, req)
			httpErr := err.(HTTPCodedError).Code()
			require.Equal(t, 400, httpErr)
		})
	})

	t.Run("unknown log_level", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/monitor?log_level=unknown", nil)
			require.NoError(t, err)
			resp := newClosableRecorder()

			// Make the request
			_, err = s.Server.AgentMonitor(resp, req)
			httpErr := err.(HTTPCodedError).Code()
			require.Equal(t, 400, httpErr)
		})
	})

	t.Run("check for specific log level", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/monitor?log_level=warn", nil)
			require.NoError(t, err)
			resp := newClosableRecorder()
			defer resp.Close()

			go func() {
				_, err = s.Server.AgentMonitor(resp, req)
				assert.NoError(t, err)
			}()

			// send the same log until monitor sink is set up
			maxLogAttempts := 10
			tried := 0
			testutil.WaitForResult(func() (bool, error) {
				if tried < maxLogAttempts {
					s.Server.logger.Warn("log that should be sent")
					tried++
				}

				got := resp.Body.String()
				want := `{"Data":"`
				if strings.Contains(got, want) {
					return true, nil
				}

				return false, fmt.Errorf("missing expected log, got: %v, want: %v", got, want)
			}, func(err error) {
				require.Fail(t, err.Error())
			})
		})
	})

	t.Run("plain output", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/monitor?log_level=debug&plain=true", nil)
			require.NoError(t, err)
			resp := newClosableRecorder()
			defer resp.Close()

			go func() {
				_, err = s.Server.AgentMonitor(resp, req)
				assert.NoError(t, err)
			}()

			// send the same log until monitor sink is set up
			maxLogAttempts := 10
			tried := 0
			testutil.WaitForResult(func() (bool, error) {
				if tried < maxLogAttempts {
					s.Server.logger.Debug("log that should be sent")
					tried++
				}

				got := resp.Body.String()
				want := `[DEBUG] http: log that should be sent`
				if strings.Contains(got, want) {
					return true, nil
				}

				return false, fmt.Errorf("missing expected log, got: %v, want: %v", got, want)
			}, func(err error) {
				require.Fail(t, err.Error())
			})
		})
	})

	t.Run("logs for a specific node", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/monitor?log_level=warn&node_id="+s.client.NodeID(), nil)
			require.NoError(t, err)
			resp := newClosableRecorder()
			defer resp.Close()

			go func() {
				_, err = s.Server.AgentMonitor(resp, req)
				assert.NoError(t, err)
			}()

			// send the same log until monitor sink is set up
			maxLogAttempts := 10
			tried := 0
			out := ""
			testutil.WaitForResult(func() (bool, error) {
				if tried < maxLogAttempts {
					s.Server.logger.Debug("log that should not be sent")
					s.Server.logger.Warn("log that should be sent")
					tried++
				}
				output, err := io.ReadAll(resp.Body)
				if err != nil {
					return false, err
				}

				out += string(output)
				want := `{"Data":"`
				if strings.Contains(out, want) {
					return true, nil
				}

				return false, fmt.Errorf("missing expected log, got: %v, want: %v", out, want)
			}, func(err error) {
				require.Fail(t, err.Error())
			})
		})
	})

	t.Run("logs for a local client with no server running on agent", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/monitor?log_level=warn", nil)
			require.NoError(t, err)
			resp := newClosableRecorder()
			defer resp.Close()

			go func() {
				// set server to nil to monitor as client
				s.Agent.server = nil
				_, err = s.Server.AgentMonitor(resp, req)
				assert.NoError(t, err)
			}()

			// send the same log until monitor sink is set up
			maxLogAttempts := 10
			tried := 0
			out := ""
			testutil.WaitForResult(func() (bool, error) {
				if tried < maxLogAttempts {
					s.Agent.logger.Warn("log that should be sent")
					tried++
				}
				output, err := io.ReadAll(resp.Body)
				if err != nil {
					return false, err
				}

				out += string(output)
				want := `{"Data":"`
				if strings.Contains(out, want) {
					return true, nil
				}

				return false, fmt.Errorf("missing expected log, got: %v, want: %v", out, want)
			}, func(err error) {
				require.Fail(t, err.Error())
			})
		})
	})
}

// Scenarios when Pprof requests should be available
// see https://github.com/hashicorp/nomad/issues/6496
// +---------------+------------------+--------+------------------+
// |   Endpoint    |  `enable_debug`  |  ACLs  |  **Available?**  |
// +---------------+------------------+--------+------------------+
// | /debug/pprof  |  unset           |  n/a   |  no              |
// | /debug/pprof  |  `true`          |  n/a   |  yes             |
// | /debug/pprof  |  `false`         |  n/a   |  no              |
// | /agent/pprof  |  unset           |  off   |  no              |
// | /agent/pprof  |  unset           |  on    |  **yes**         |
// | /agent/pprof  |  `true`          |  off   |  yes             |
// | /agent/pprof  |  `false`         |  on    |  **yes**         |
// +---------------+------------------+--------+------------------+
func TestAgent_PprofRequest_Permissions(t *testing.T) {
	ci.Parallel(t)

	trueP, falseP := pointer.Of(true), pointer.Of(false)
	cases := []struct {
		acl   *bool
		debug *bool
		ok    bool
	}{
		// manually set to false because test helpers
		// enable to true by default
		// enableDebug:       pointer.Of(false),
		{debug: nil, ok: false},
		{debug: trueP, ok: true},
		{debug: falseP, ok: false},
		{debug: falseP, acl: falseP, ok: false},
		{acl: trueP, ok: true},
		{acl: falseP, debug: trueP, ok: true},
		{debug: falseP, acl: trueP, ok: true},
	}

	for _, tc := range cases {
		ptrToStr := func(val *bool) string {
			if val == nil {
				return "unset"
			} else if *val == true {
				return "true"
			} else {
				return "false"
			}
		}

		t.Run(
			fmt.Sprintf("debug %s, acl %s",
				ptrToStr(tc.debug),
				ptrToStr(tc.acl)),
			func(t *testing.T) {
				cb := func(c *Config) {
					if tc.acl != nil {
						c.ACL.Enabled = *tc.acl
					}
					if tc.debug == nil {
						var nodebug bool
						c.EnableDebug = nodebug
					} else {
						c.EnableDebug = *tc.debug
					}
				}

				httpTest(t, cb, func(s *TestAgent) {
					state := s.Agent.server.State()
					url := "/v1/agent/pprof/cmdline"
					req, err := http.NewRequest(http.MethodGet, url, nil)
					require.NoError(t, err)
					respW := httptest.NewRecorder()

					if tc.acl != nil && *tc.acl {
						token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.AgentPolicy(acl.PolicyWrite))
						setToken(req, token)
					}

					resp, err := s.Server.AgentPprofRequest(respW, req)
					if tc.ok {
						require.NoError(t, err)
						require.NotNil(t, resp)
					} else {
						require.Error(t, err)
						require.Equal(t, structs.ErrPermissionDenied.Error(), err.Error())
					}
				})
			})
	}
}

func TestAgent_PprofRequest(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc        string
		url         string
		addNodeID   bool
		addServerID bool
		expectedErr string
		clientOnly  bool
	}{
		{
			desc: "cmdline local server request",
			url:  "/v1/agent/pprof/cmdline",
		},
		{
			desc:       "cmdline local node request",
			url:        "/v1/agent/pprof/cmdline",
			clientOnly: true,
		},
		{
			desc:      "cmdline node request",
			url:       "/v1/agent/pprof/cmdline",
			addNodeID: true,
		},
		{
			desc:        "cmdline server request",
			url:         "/v1/agent/pprof/cmdline",
			addServerID: true,
		},
		{
			desc:        "invalid server request",
			url:         "/v1/agent/pprof/unknown",
			addServerID: true,
			expectedErr: "RPC Error:: 404,Pprof profile not found profile: unknown",
		},
		{
			desc:      "cpu profile request",
			url:       "/v1/agent/pprof/profile",
			addNodeID: true,
		},
		{
			desc:      "trace request",
			url:       "/v1/agent/pprof/trace",
			addNodeID: true,
		},
		{
			desc:      "pprof lookup request",
			url:       "/v1/agent/pprof/goroutine",
			addNodeID: true,
		},
		{
			desc:        "unknown pprof lookup request",
			url:         "/v1/agent/pprof/latency",
			addNodeID:   true,
			expectedErr: "RPC Error:: 404,Pprof profile not found profile: latency",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			httpTest(t, nil, func(s *TestAgent) {

				// add node or server id query param
				url := tc.url
				if tc.addNodeID {
					url = url + "?node_id=" + s.client.NodeID()
				} else if tc.addServerID {
					url = url + "?server_id=" + s.server.LocalMember().Name
				}

				if tc.clientOnly {
					s.Agent.server = nil
				}

				req, err := http.NewRequest(http.MethodGet, url, nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				resp, err := s.Server.AgentPprofRequest(respW, req)

				if tc.expectedErr != "" {
					require.Error(t, err)
					require.EqualError(t, err, tc.expectedErr)
				} else {
					require.NoError(t, err)
					require.NotNil(t, resp)
				}
			})
		})
	}
}

type closableRecorder struct {
	*httptest.ResponseRecorder
	closer chan bool
}

func newClosableRecorder() *closableRecorder {
	r := httptest.NewRecorder()
	closer := make(chan bool)
	return &closableRecorder{r, closer}
}

func (r *closableRecorder) Close() {
	close(r.closer)
}

func (r *closableRecorder) CloseNotify() <-chan bool {
	return r.closer
}

func TestHTTP_AgentForceLeave(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/agent/force-leave?node=foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		_, err = s.Server.AgentForceLeaveRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestHTTP_AgentForceLeave_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/agent/force-leave?node=foo", nil)
		require.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.AgentForceLeaveRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			_, err := s.Server.AgentForceLeaveRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.AgentPolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err := s.Server.AgentForceLeaveRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
		}

		// Try request with a root token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			_, err := s.Server.AgentForceLeaveRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
		}
	})
}

func TestHTTP_AgentSetServers(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		addr := s.Config.AdvertiseAddrs.RPC
		testutil.WaitForResult(func() (bool, error) {
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err != nil {
				return false, err
			}
			defer conn.Close()

			// Write the Nomad RPC byte to set the mode
			if _, err := conn.Write([]byte{byte(pool.RpcNomad)}); err != nil {
				return false, err
			}

			codec := pool.NewClientCodec(conn)
			args := &structs.GenericRequest{}
			var leader string
			err = msgpackrpc.CallWithCodec(codec, "Status.Leader", args, &leader)
			return leader != "", err
		}, func(err error) {
			t.Fatalf("failed to find leader: %v", err)
		})

		// Create the request
		req, err := http.NewRequest(http.MethodPut, "/v1/agent/servers", nil)
		require.Nil(err)

		// Send the request
		respW := httptest.NewRecorder()
		_, err = s.Server.AgentServersRequest(respW, req)
		require.NotNil(err)
		require.Contains(err.Error(), "missing server address")

		// Create a valid request
		req, err = http.NewRequest(http.MethodPut, "/v1/agent/servers?address=127.0.0.1%3A4647&address=127.0.0.2%3A4647&address=127.0.0.3%3A4647", nil)
		require.Nil(err)

		// Send the request which should fail
		respW = httptest.NewRecorder()
		_, err = s.Server.AgentServersRequest(respW, req)
		require.NotNil(err)

		// Retrieve the servers again
		req, err = http.NewRequest(http.MethodGet, "/v1/agent/servers", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		// Make the request and check the result
		expected := []string{
			s.GetConfig().AdvertiseAddrs.RPC,
		}
		out, err := s.Server.AgentServersRequest(respW, req)
		require.Nil(err)
		servers := out.([]string)
		require.Len(servers, len(expected))
		require.Equal(expected, servers)
	})
}

func TestHTTP_AgentSetServers_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		addr := s.Config.AdvertiseAddrs.RPC
		testutil.WaitForResult(func() (bool, error) {
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err != nil {
				return false, err
			}
			defer conn.Close()

			// Write the Consul RPC byte to set the mode
			if _, err := conn.Write([]byte{byte(pool.RpcNomad)}); err != nil {
				return false, err
			}

			codec := pool.NewClientCodec(conn)
			args := &structs.GenericRequest{}
			var leader string
			err = msgpackrpc.CallWithCodec(codec, "Status.Leader", args, &leader)
			return leader != "", err
		}, func(err error) {
			t.Fatalf("failed to find leader: %v", err)
		})

		// Make the HTTP request
		path := fmt.Sprintf("/v1/agent/servers?address=%s", url.QueryEscape(s.GetConfig().AdvertiseAddrs.RPC))
		req, err := http.NewRequest(http.MethodPut, path, nil)
		require.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.AgentServersRequest(respW, req)
			require.NotNil(err)
			require.ErrorContains(err, structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			_, err := s.Server.AgentServersRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.AgentPolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err := s.Server.AgentServersRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
		}

		// Try request with a root token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			_, err := s.Server.AgentServersRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
		}
	})
}

func TestHTTP_AgentListServers_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Create list request
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/servers", nil)
		require.Nil(err)

		expected := []string{
			s.GetConfig().AdvertiseAddrs.RPC,
		}

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.AgentServersRequest(respW, req)
			require.NotNil(err)
			require.ErrorContains(err, structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			_, err := s.Server.AgentServersRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Wait for client to have a server
		testutil.WaitForResult(func() (bool, error) {
			return len(s.client.GetServers()) != 0, fmt.Errorf("no servers")
		}, func(err error) {
			t.Fatal(err)
		})

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.AgentPolicy(acl.PolicyRead))
			setToken(req, token)
			out, err := s.Server.AgentServersRequest(respW, req)
			require.Nil(err)
			servers := out.([]string)
			require.Len(servers, len(expected))
			require.Equal(expected, servers)
		}

		// Try request with a root token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			out, err := s.Server.AgentServersRequest(respW, req)
			require.Nil(err)
			servers := out.([]string)
			require.Len(servers, len(expected))
			require.Equal(expected, servers)
		}
	})
}

func TestHTTP_AgentListKeys(t *testing.T) {
	ci.Parallel(t)

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="

	httpTest(t, func(c *Config) {
		c.Server.EncryptKey = key1
	}, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/keyring/list", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW := httptest.NewRecorder()

		out, err := s.Server.KeyringOperationRequest(respW, req)
		require.NoError(t, err)
		kresp := out.(structs.KeyringResponse)
		require.Len(t, kresp.Keys, 1)
	})
}

func TestHTTP_AgentListKeys_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="

	cb := func(c *Config) {
		c.Server.EncryptKey = key1
	}

	httpACLTest(t, cb, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/keyring/list", nil)
		require.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.KeyringOperationRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.AgentPolicy(acl.PolicyRead))
			setToken(req, token)
			_, err := s.Server.KeyringOperationRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.AgentPolicy(acl.PolicyWrite))
			setToken(req, token)
			out, err := s.Server.KeyringOperationRequest(respW, req)
			require.Nil(err)
			kresp := out.(structs.KeyringResponse)
			require.Len(kresp.Keys, 1)
			require.Contains(kresp.Keys, key1)
		}

		// Try request with a root token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			out, err := s.Server.KeyringOperationRequest(respW, req)
			require.Nil(err)
			kresp := out.(structs.KeyringResponse)
			require.Len(kresp.Keys, 1)
			require.Contains(kresp.Keys, key1)
		}
	})
}

func TestHTTP_AgentInstallKey(t *testing.T) {
	ci.Parallel(t)

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="
	key2 := "wH1Bn9hlJ0emgWB1JttVRA=="

	httpTest(t, func(c *Config) {
		c.Server.EncryptKey = key1
	}, func(s *TestAgent) {
		b, err := json.Marshal(&structs.KeyringRequest{Key: key2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/keyring/install", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.KeyringOperationRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		req, err = http.NewRequest(http.MethodGet, "/v1/agent/keyring/list", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW = httptest.NewRecorder()

		out, err := s.Server.KeyringOperationRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		kresp := out.(structs.KeyringResponse)
		if len(kresp.Keys) != 2 {
			t.Fatalf("bad: %v", kresp)
		}
	})
}

func TestHTTP_AgentRemoveKey(t *testing.T) {
	ci.Parallel(t)

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="
	key2 := "wH1Bn9hlJ0emgWB1JttVRA=="

	httpTest(t, func(c *Config) {
		c.Server.EncryptKey = key1
	}, func(s *TestAgent) {
		b, err := json.Marshal(&structs.KeyringRequest{Key: key2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		req, err := http.NewRequest(http.MethodGet, "/v1/agent/keyring/install", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW := httptest.NewRecorder()
		_, err = s.Server.KeyringOperationRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		req, err = http.NewRequest(http.MethodGet, "/v1/agent/keyring/remove", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW = httptest.NewRecorder()
		if _, err = s.Server.KeyringOperationRequest(respW, req); err != nil {
			t.Fatalf("err: %s", err)
		}

		req, err = http.NewRequest(http.MethodGet, "/v1/agent/keyring/list", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW = httptest.NewRecorder()
		out, err := s.Server.KeyringOperationRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		kresp := out.(structs.KeyringResponse)
		if len(kresp.Keys) != 1 {
			t.Fatalf("bad: %v", kresp)
		}
	})
}

func TestHTTP_AgentHealth_Ok(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Enable ACLs to ensure they're not enforced
	httpACLTest(t, nil, func(s *TestAgent) {
		// No ?type=
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/health", nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			healthI, err := s.Server.HealthRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
			require.NotNil(healthI)
			health := healthI.(*healthResponse)
			require.NotNil(health.Client)
			require.True(health.Client.Ok)
			require.Equal("ok", health.Client.Message)
			require.NotNil(health.Server)
			require.True(health.Server.Ok)
			require.Equal("ok", health.Server.Message)
		}

		// type=client
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/health?type=client", nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			healthI, err := s.Server.HealthRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
			require.NotNil(healthI)
			health := healthI.(*healthResponse)
			require.NotNil(health.Client)
			require.True(health.Client.Ok)
			require.Equal("ok", health.Client.Message)
			require.Nil(health.Server)
		}

		// type=server
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/health?type=server", nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			healthI, err := s.Server.HealthRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
			require.NotNil(healthI)
			health := healthI.(*healthResponse)
			require.NotNil(health.Server)
			require.True(health.Server.Ok)
			require.Equal("ok", health.Server.Message)
			require.Nil(health.Client)
		}

		// type=client&type=server
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/health?type=client&type=server", nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			healthI, err := s.Server.HealthRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
			require.NotNil(healthI)
			health := healthI.(*healthResponse)
			require.NotNil(health.Client)
			require.True(health.Client.Ok)
			require.Equal("ok", health.Client.Message)
			require.NotNil(health.Server)
			require.True(health.Server.Ok)
			require.Equal("ok", health.Server.Message)
		}
	})
}

func TestHTTP_AgentHealth_BadServer(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	serverAgent := NewTestAgent(t, "server", nil)
	defer serverAgent.Shutdown()

	s := makeHTTPServer(t, func(c *Config) {
		// Disable server to make server health unhealthy if requested
		c.Server.Enabled = false
		c.Client.Servers = []string{fmt.Sprintf("localhost:%d", serverAgent.Config.Ports.RPC)}
	})
	defer s.Shutdown()

	// No ?type= means server is just skipped
	{
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/health", nil)
		require.Nil(err)

		respW := httptest.NewRecorder()
		healthI, err := s.Server.HealthRequest(respW, req)
		require.Nil(err)
		require.Equal(http.StatusOK, respW.Code)
		require.NotNil(healthI)
		health := healthI.(*healthResponse)
		require.NotNil(health.Client)
		require.True(health.Client.Ok)
		require.Equal("ok", health.Client.Message)
		require.Nil(health.Server)
	}

	// type=server means server is considered unhealthy
	{
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/health?type=server", nil)
		require.Nil(err)

		respW := httptest.NewRecorder()
		_, err = s.Server.HealthRequest(respW, req)
		require.NotNil(err)
		httpErr, ok := err.(HTTPCodedError)
		require.True(ok)
		require.Equal(500, httpErr.Code())
		require.Equal(`{"server":{"ok":false,"message":"server not enabled"}}`, err.Error())
	}
}

func TestHTTP_AgentHealth_BadClient(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Disable client to make server unhealthy if requested
	cb := func(c *Config) {
		c.Client.Enabled = false
	}

	// Enable ACLs to ensure they're not enforced
	httpACLTest(t, cb, func(s *TestAgent) {
		// No ?type= means client is just skipped
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/health", nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			healthI, err := s.Server.HealthRequest(respW, req)
			require.Nil(err)
			require.Equal(http.StatusOK, respW.Code)
			require.NotNil(healthI)
			health := healthI.(*healthResponse)
			require.NotNil(health.Server)
			require.True(health.Server.Ok)
			require.Equal("ok", health.Server.Message)
			require.Nil(health.Client)
		}

		// type=client means client is considered unhealthy
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/agent/health?type=client", nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			_, err = s.Server.HealthRequest(respW, req)
			require.NotNil(err)
			httpErr, ok := err.(HTTPCodedError)
			require.True(ok)
			require.Equal(500, httpErr.Code())
			require.Equal(`{"client":{"ok":false,"message":"client not enabled"}}`, err.Error())
		}
	})
}

var (
	errorPipe = &net.OpError{
		Op:     "write",
		Net:    "tcp",
		Source: &net.TCPAddr{},
		Addr:   &net.TCPAddr{},
		Err: &os.SyscallError{
			Syscall: "write",
			Err:     syscall.EPIPE,
		},
	}
)

// fakeRW is a fake response writer to ease polling streaming responses in a
// data-race-free way.
type fakeRW struct {
	Code      int
	HeaderMap http.Header
	buf       *bytes.Buffer
	closed    bool
	mu        sync.Mutex

	// Written is ticked whenever a Write occurs and on WriteHeaders if it
	// is explicitly called
	Written chan int

	// ClosedErr is the error Write will return once the writer is closed.
	// Defaults to EPIPE. Must not be mutated concurrently with writes.
	ClosedErr error
}

// Header is for setting headers before writing a response. Tests should check
// the HeaderMap field directly.
func (f *fakeRW) Header() http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Code != 0 {
		panic("cannot set headers after WriteHeader has been called")
	}

	return f.HeaderMap
}

func (f *fakeRW) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		// Mimic an EPIPE error
		return 0, f.ClosedErr
	}

	if f.Code == 0 {
		f.Code = 200
	}

	n, err := f.buf.Write(p)
	select {
	case f.Written <- 1:
	default:
	}
	return n, err
}

// WriteHeader sets Code and FinalHeaders
func (f *fakeRW) WriteHeader(statusCode int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Code != 0 {
		panic("cannot call WriteHeader more than once")
	}

	f.Code = statusCode
	select {
	case f.Written <- 1:
	default:
	}
}

// Bytes returns the body bytes written to the buffer. Safe for calling
// concurrent with writes.
func (f *fakeRW) Bytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.buf.Bytes()
}

// Close the writer causing an EPIPE error on future writes. Safe to call
// concurrently with other methods. Safe to call more than once.
func (f *fakeRW) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

func NewFakeRW() *fakeRW {
	return &fakeRW{
		HeaderMap: make(map[string][]string),
		buf:       &bytes.Buffer{},
		Written:   make(chan int, 1),
		ClosedErr: errorPipe,
	}
}

// TestHTTP_XSS_Monitor asserts /v1/agent/monitor is safe against XSS attacks
// even when log output contains HTML+Javascript.
func TestHTTP_XSS_Monitor(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name    string
		Logline string
		JSON    bool
	}{
		{
			Name:    "Plain",
			Logline: "--TEST 123--",
			JSON:    false,
		},
		{
			Name:    "JSON",
			Logline: "--TEST 123--",
			JSON:    true,
		},
		{
			Name:    "XSSPlain",
			Logline: "<script>alert(document.domain);</script>",
			JSON:    false,
		},
		{
			Name:    "XSSJson",
			Logline: "<script>alert(document.domain);</script>",
			JSON:    true,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			ci.Parallel(t)
			s := makeHTTPServer(t, nil)
			defer s.Shutdown()

			path := fmt.Sprintf("%s/v1/agent/monitor?error_level=error&plain=%t", s.HTTPAddr(), !tc.JSON)
			req, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			resp := NewFakeRW()
			closedErr := errors.New("sentinel error")
			resp.ClosedErr = closedErr
			defer resp.Close()

			errCh := make(chan error, 1)
			go func() {
				_, err := s.Server.AgentMonitor(resp, req)
				errCh <- err
			}()

			deadline := time.After(3 * time.Second)

		OUTER:
			for {
				// Log a needle and look for it in the response haystack
				s.Server.logger.Error(tc.Logline)

				select {
				case <-time.After(30 * time.Millisecond):
					// Give AgentMonitor handler goroutine time to start
				case <-resp.Written:
					// Something was written, check it
				case <-deadline:
					t.Fatalf("timed out waiting for expected log line; body:\n%s", string(resp.Bytes()))
				case err := <-errCh:
					t.Fatalf("AgentMonitor exited unexpectedly: err=%v", err)
				}

				if !tc.JSON {
					if bytes.Contains(resp.Bytes(), []byte(tc.Logline)) {
						// Found needle!
						break
					} else {
						// Try again
						continue
					}
				}

				// Decode JSON
				r := bytes.NewReader(resp.Bytes())
				dec := json.NewDecoder(r)
				for {
					data := struct{ Data []byte }{}
					if err := dec.Decode(&data); err != nil {
						// Probably a partial write, continue
						continue OUTER
					}

					if bytes.Contains(data.Data, []byte(tc.Logline)) {
						// Found needle!
						break OUTER
					}
				}

			}

			// Assert default logs are application/json
			ct := "text/plain"
			if tc.JSON {
				ct = "application/json"
			}
			require.Equal(t, []string{ct}, resp.HeaderMap.Values("Content-Type"))

			// Close response writer and log to make AgentMonitor exit
			resp.Close()
			s.Server.logger.Error("log again to force a write that detects the closed connection")
			select {
			case err := <-errCh:
				require.EqualError(t, closedErr, err.Error())
			case <-deadline:
				t.Fatalf("timed out waiting for closing error from handler")
			}
		})
	}
}

// ----------------------------
// SchedulerWorkerInfoAPI tests
// ----------------------------
type schedulerWorkerAPITest_testCase struct {
	name              string // test case name
	request           schedulerWorkerAPITest_testRequest
	whenACLNotEnabled schedulerWorkerAPITest_testExpect
	whenACLEnabled    schedulerWorkerAPITest_testExpect
}

type schedulerWorkerAPITest_testRequest struct {
	verb        string
	aclToken    string
	requestBody string
}

type schedulerWorkerAPITest_testExpect struct {
	statusCode int
	response   interface{}
	err        error
	isError    bool
}

func (te schedulerWorkerAPITest_testExpect) Code() int {
	return te.statusCode
}

func schedulerWorkerInfoTest_testCases() []schedulerWorkerAPITest_testCase {
	forbidden := schedulerWorkerAPITest_testExpect{
		statusCode: http.StatusForbidden,
		response:   structs.ErrPermissionDenied.Error(),
		isError:    true,
	}
	invalidMethod := schedulerWorkerAPITest_testExpect{
		statusCode: http.StatusMethodNotAllowed,
		response:   ErrInvalidMethod,
		isError:    true,
	}
	success := schedulerWorkerAPITest_testExpect{
		statusCode: http.StatusOK,
		response: &api.AgentSchedulerWorkersInfo{
			Schedulers: []api.AgentSchedulerWorkerInfo{
				{
					ID:                "9b3713e0-6f74-0e1b-3b3e-d94f0c22dbf9",
					EnabledSchedulers: []string{"_core", "batch"},
					Started:           "2021-12-10 22:13:12.595366 -0500 EST m=+0.039016232",
					Status:            "Pausing",
					WorkloadStatus:    "WaitingToDequeue",
				},
				{
					ID:                "ebda23e2-7f68-0c82-f0b2-f91d4581094d",
					EnabledSchedulers: []string{"_core", "batch"},
					Started:           "2021-12-10 22:13:12.595478 -0500 EST m=+0.039127886",
					Status:            "Pausing",
					WorkloadStatus:    "WaitingToDequeue",
				},
				{
					ID:                "b3869c9b-64ff-686c-a003-e7d059d3a573",
					EnabledSchedulers: []string{"_core", "batch"},
					Started:           "2021-12-10 22:13:12.595501 -0500 EST m=+0.039151276",
					Status:            "Pausing",
					WorkloadStatus:    "WaitingToDequeue",
				},
				{
					ID:                "cc5907c0-552e-bf36-0ca1-f150af7273c2",
					EnabledSchedulers: []string{"_core", "batch"},
					Started:           "2021-12-10 22:13:12.595691 -0500 EST m=+0.039341541",
					Status:            "Starting",
					WorkloadStatus:    "WaitingToDequeue",
				},
			},
		},
	}
	return []schedulerWorkerAPITest_testCase{
		{
			name: "bad verb",
			request: schedulerWorkerAPITest_testRequest{
				verb:        "FOO",
				aclToken:    "",
				requestBody: "",
			},
			whenACLNotEnabled: invalidMethod,
			whenACLEnabled:    invalidMethod,
		},
		{
			name: "get without token",
			request: schedulerWorkerAPITest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "",
				requestBody: "",
			},
			whenACLNotEnabled: success,
			whenACLEnabled:    forbidden,
		},
		{
			name: "get with management token",
			request: schedulerWorkerAPITest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "management",
				requestBody: "",
			},
			whenACLNotEnabled: success,
			whenACLEnabled:    success,
		},
		{
			name: "get with read token",
			request: schedulerWorkerAPITest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "agent_read",
				requestBody: "",
			},
			whenACLNotEnabled: success,
			whenACLEnabled:    success,
		},
		{
			name: "get with invalid token",
			request: schedulerWorkerAPITest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "node_write",
				requestBody: "",
			},
			whenACLNotEnabled: success,
			whenACLEnabled:    forbidden,
		},
	}
}

func TestHTTP_AgentSchedulerWorkerInfoRequest(t *testing.T) {
	ci.Parallel(t)

	configFn := func(c *Config) {
		var numSchedulers = 4
		c.Server.NumSchedulers = &numSchedulers
		c.Server.EnabledSchedulers = []string{"_core", "batch"}
		c.Client.Enabled = false
	}

	for _, runACL := range []string{"no_acl", "acl"} {
		t.Run(runACL, func(t *testing.T) {
			tests := func(s *TestAgent) {
				testingACLS := s.Config.ACL.Enabled
				var tokens map[string]*structs.ACLToken
				if s.Config.ACL.Enabled {
					state := s.Agent.server.State()
					tokens = make(map[string]*structs.ACLToken)

					tokens["management"] = s.RootToken
					tokens["agent_read"] = mock.CreatePolicyAndToken(t, state, 1005, "agent_read", mock.AgentPolicy(acl.PolicyRead))
					tokens["agent_write"] = mock.CreatePolicyAndToken(t, state, 1007, "agent_write", mock.AgentPolicy(acl.PolicyWrite))
					tokens["node_write"] = mock.CreatePolicyAndToken(t, state, 1009, "node_write", mock.NodePolicy(acl.PolicyWrite))
				}

				for _, tc := range schedulerWorkerInfoTest_testCases() {
					t.Run(tc.name, func(t *testing.T) {
						req, err := http.NewRequest(tc.request.verb, "/v1/agent/schedulers", bytes.NewReader([]byte(tc.request.requestBody)))
						if testingACLS && tc.request.aclToken != "" {
							setToken(req, tokens[tc.request.aclToken])
						}
						require.NoError(t, err)
						respW := httptest.NewRecorder()
						workerInfoResp, err := s.Server.AgentSchedulerWorkerInfoRequest(respW, req)

						expected := tc.whenACLNotEnabled
						if testingACLS {
							expected = tc.whenACLEnabled
						}

						if expected.isError {
							require.Error(t, err)
							codedErr, ok := err.(HTTPCodedError)
							require.True(t, ok, "expected a HTTPCodedError")
							require.Equal(t, expected.Code(), codedErr.Code())
							require.Equal(t, expected.response, codedErr.Error())
							return
						}

						require.NoError(t, err)
						workerInfo, ok := workerInfoResp.(*api.AgentSchedulerWorkersInfo)
						require.True(t, ok, "expected an *AgentSchedulersWorkersInfo. received:%s", reflect.TypeOf(workerInfoResp))

						expectWorkerInfo, ok := expected.response.(*api.AgentSchedulerWorkersInfo)
						require.True(t, ok, "error casting test case to *AgentSchedulersWorkersInfo. received:%s", reflect.TypeOf(workerInfoResp))

						var schedCount int = *s.Config.Server.NumSchedulers
						require.Equal(t, schedCount, len(workerInfo.Schedulers), "must match num_schedulers")
						require.Equal(t, len(expectWorkerInfo.Schedulers), len(workerInfo.Schedulers), "lengths must match")

						for i, info := range expectWorkerInfo.Schedulers {
							require.ElementsMatch(t, info.EnabledSchedulers, workerInfo.Schedulers[i].EnabledSchedulers)
						}
					})
				}
			}

			if runACL == "acl" {
				httpACLTest(t, configFn, tests)
			} else {
				httpTest(t, configFn, tests)
			}
		})
	}
}

// ----------------------------
// SchedulerWorkerConfigAPI tests
// ----------------------------
type scheduleWorkerConfigTest_workerRequestTest struct {
	name              string // test case name
	request           schedulerWorkerConfigTest_testRequest
	whenACLNotEnabled schedulerWorkerConfigTest_testExpect
	whenACLEnabled    schedulerWorkerConfigTest_testExpect
}
type schedulerWorkerConfigTest_testRequest struct {
	verb        string
	aclToken    string
	requestBody string
}
type schedulerWorkerConfigTest_testExpect struct {
	expectedResponseCode int
	expectedResponse     interface{}
}

// These test cases are run for both the ACL and Non-ACL enabled servers. When
// ACLS are not enabled, the request.aclTokens are ignored.
func schedulerWorkerConfigTest_testCases() []scheduleWorkerConfigTest_workerRequestTest {
	forbidden := schedulerWorkerConfigTest_testExpect{
		expectedResponseCode: http.StatusForbidden,
		expectedResponse:     structs.ErrPermissionDenied.Error(),
	}
	invalidMethod := schedulerWorkerConfigTest_testExpect{
		expectedResponseCode: http.StatusMethodNotAllowed,
		expectedResponse:     ErrInvalidMethod,
	}
	invalidRequest := schedulerWorkerConfigTest_testExpect{
		expectedResponseCode: http.StatusBadRequest,
		expectedResponse:     "Invalid request",
	}
	success1 := schedulerWorkerConfigTest_testExpect{
		expectedResponseCode: http.StatusOK,
		expectedResponse:     &api.AgentSchedulerWorkerConfigResponse{EnabledSchedulers: []string{"_core", "batch"}, NumSchedulers: 8},
	}

	success2 := schedulerWorkerConfigTest_testExpect{
		expectedResponseCode: http.StatusOK,
		expectedResponse:     &api.AgentSchedulerWorkerConfigResponse{EnabledSchedulers: []string{"_core", "batch"}, NumSchedulers: 9},
	}

	return []scheduleWorkerConfigTest_workerRequestTest{
		{
			name: "bad verb",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        "FOO",
				aclToken:    "",
				requestBody: "",
			},
			whenACLNotEnabled: invalidMethod,
			whenACLEnabled:    invalidMethod,
		},
		{
			name: "get without token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "",
				requestBody: "",
			},
			whenACLNotEnabled: success1,
			whenACLEnabled:    forbidden,
		},
		{
			name: "get with management token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "management",
				requestBody: "",
			},
			whenACLNotEnabled: success1,
			whenACLEnabled:    success1,
		},
		{
			name: "get with read token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "agent_read",
				requestBody: "",
			},
			whenACLNotEnabled: success1,
			whenACLEnabled:    success1,
		},
		{
			name: "get with write token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodGet,
				aclToken:    "agent_write",
				requestBody: "",
			},
			whenACLNotEnabled: success1,
			whenACLEnabled:    success1,
		},
		{
			name: "post with no token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPost,
				aclToken:    "",
				requestBody: `{"num_schedulers":9,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: success2,
			whenACLEnabled:    forbidden,
		},
		{
			name: "put with no token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPut,
				aclToken:    "",
				requestBody: `{"num_schedulers":8,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: success1,
			whenACLEnabled:    forbidden,
		},
		{
			name: "post with invalid token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPost,
				aclToken:    "node_write",
				requestBody: `{"num_schedulers":9,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: success2,
			whenACLEnabled:    forbidden,
		},
		{
			name: "put with invalid token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPut,
				aclToken:    "node_write",
				requestBody: `{"num_schedulers":8,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: success1,
			whenACLEnabled:    forbidden,
		},
		{
			name: "post with valid token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPost,
				aclToken:    "agent_write",
				requestBody: `{"num_schedulers":9,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: success2,
			whenACLEnabled:    success2,
		},
		{
			name: "put with valid token",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPut,
				aclToken:    "agent_write",
				requestBody: `{"num_schedulers":8,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: success1,
			whenACLEnabled:    success1,
		},
		{
			name: "post with good token and bad value",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPost,
				aclToken:    "agent_write",
				requestBody: `{"num_schedulers":-1,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: invalidRequest,
			whenACLEnabled:    invalidRequest,
		},
		{
			name: "post with bad token and bad value",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPost,
				aclToken:    "node_write",
				requestBody: `{"num_schedulers":-1,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: invalidRequest,
			whenACLEnabled:    forbidden,
		},
		{
			name: "put with good token and bad value",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPut,
				aclToken:    "agent_write",
				requestBody: `{"num_schedulers":-1,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: invalidRequest,
			whenACLEnabled:    invalidRequest,
		},
		{
			name: "put with bad token and bad value",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPut,
				aclToken:    "node_write",
				requestBody: `{"num_schedulers":-1,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: invalidRequest,
			whenACLEnabled:    forbidden,
		},
		{
			name: "post with bad json",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPost,
				aclToken:    "agent_write",
				requestBody: `{num_schedulers:-1,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: invalidRequest,
			whenACLEnabled:    invalidRequest,
		},
		{
			name: "put with bad json",
			request: schedulerWorkerConfigTest_testRequest{
				verb:        http.MethodPut,
				aclToken:    "agent_write",
				requestBody: `{num_schedulers:-1,"enabled_schedulers":["_core", "batch"]}`,
			},
			whenACLNotEnabled: invalidRequest,
			whenACLEnabled:    invalidRequest,
		},
	}
}

func TestHTTP_AgentSchedulerWorkerConfigRequest_NoACL(t *testing.T) {
	ci.Parallel(t)

	configFn := func(c *Config) {
		var numSchedulers = 8
		c.Server.NumSchedulers = &numSchedulers
		c.Server.EnabledSchedulers = []string{"_core", "batch"}
		c.Client.Enabled = false
	}
	testFn := func(s *TestAgent) {
		for _, tc := range schedulerWorkerConfigTest_testCases() {
			t.Run(tc.name, func(t *testing.T) {

				req, err := http.NewRequest(tc.request.verb, "/v1/agent/schedulers/config", bytes.NewReader([]byte(tc.request.requestBody)))
				require.NoError(t, err)
				respW := httptest.NewRecorder()
				workersI, err := s.Server.AgentSchedulerWorkerConfigRequest(respW, req)

				switch tc.whenACLNotEnabled.expectedResponseCode {
				case http.StatusBadRequest, http.StatusForbidden, http.StatusMethodNotAllowed:
					schedulerWorkerTest_parseError(t, false, tc, workersI, err)
				case http.StatusOK:
					schedulerWorkerTest_parseSuccess(t, false, tc, workersI, err)
				default:
					require.Failf(t, "unexpected status code", "code: %v", tc.whenACLNotEnabled.expectedResponseCode)
				}
			})
		}
	}

	httpTest(t, configFn, testFn)
}

func TestHTTP_AgentSchedulerWorkerConfigRequest_ACL(t *testing.T) {
	ci.Parallel(t)

	configFn := func(c *Config) {
		var numSchedulers = 8
		c.Server.NumSchedulers = &numSchedulers
		c.Server.EnabledSchedulers = []string{"_core", "batch"}
		c.Client.Enabled = false
	}

	tests := func(s *TestAgent) {
		state := s.Agent.server.State()
		var tokens map[string]*structs.ACLToken = make(map[string]*structs.ACLToken)

		tokens["management"] = s.RootToken
		tokens["agent_read"] = mock.CreatePolicyAndToken(t, state, 1005, "agent_read", mock.AgentPolicy(acl.PolicyRead))
		tokens["agent_write"] = mock.CreatePolicyAndToken(t, state, 1007, "agent_write", mock.AgentPolicy(acl.PolicyWrite))
		tokens["node_write"] = mock.CreatePolicyAndToken(t, state, 1009, "node_write", mock.NodePolicy(acl.PolicyWrite))

		for _, tc := range schedulerWorkerConfigTest_testCases() {
			t.Run(tc.name, func(t *testing.T) {

				req, err := http.NewRequest(tc.request.verb, "/v1/agent/schedulers", bytes.NewReader([]byte(tc.request.requestBody)))
				if tc.request.aclToken != "" {
					setToken(req, tokens[tc.request.aclToken])
				}
				require.NoError(t, err)
				respW := httptest.NewRecorder()
				workersI, err := s.Server.AgentSchedulerWorkerConfigRequest(respW, req)

				switch tc.whenACLEnabled.expectedResponseCode {
				case http.StatusOK:
					schedulerWorkerTest_parseSuccess(t, true, tc, workersI, err)
				case http.StatusBadRequest, http.StatusForbidden, http.StatusMethodNotAllowed:
					schedulerWorkerTest_parseError(t, true, tc, workersI, err)
				default:
					require.Failf(t, "unexpected status code", "code: %v", tc.whenACLEnabled.expectedResponseCode)
				}
			})
		}
	}

	httpACLTest(t, configFn, tests)
}

func schedulerWorkerTest_parseSuccess(t *testing.T, isACLEnabled bool, tc scheduleWorkerConfigTest_workerRequestTest, workersI interface{}, err error) {
	require.NoError(t, err)
	require.NotNil(t, workersI)

	testExpect := tc.whenACLNotEnabled
	if isACLEnabled {
		testExpect = tc.whenACLNotEnabled
	}

	// test into the response when we expect an okay
	tcConfig, ok := testExpect.expectedResponse.(*api.AgentSchedulerWorkerConfigResponse)
	require.True(t, ok, "expected response malformed - this is an issue with a test case.")

	workersConfig, ok := workersI.(*api.AgentSchedulerWorkerConfigResponse)
	require.True(t, ok, "response can not cast to an agentSchedulerWorkerConfig")
	require.NotNil(t, workersConfig)

	require.Equal(t, tcConfig.NumSchedulers, workersConfig.NumSchedulers)
	require.ElementsMatch(t, tcConfig.EnabledSchedulers, workersConfig.EnabledSchedulers)
}

// schedulerWorkerTest_parseError parses the error response given
// from the API call to make sure that it's a coded error and is the
// expected value from the test case
func schedulerWorkerTest_parseError(t *testing.T, isACLEnabled bool, tc scheduleWorkerConfigTest_workerRequestTest, workersI interface{}, err error) {
	require.Error(t, err)
	require.Nil(t, workersI)

	codedError, ok := err.(HTTPCodedError)
	require.True(t, ok, "expected an HTTPCodedError")
	testExpect := tc.whenACLNotEnabled

	if isACLEnabled {
		testExpect = tc.whenACLEnabled
	}

	require.Equal(t, testExpect.expectedResponseCode, codedError.Code())
	// this is a relaxed test to allow us to not have to create a case
	// for concatenated error strings.
	require.Contains(t, codedError.Error(), testExpect.expectedResponse)
}

func TestHTTP_AgentSchedulerWorkerInfoRequest_Client(t *testing.T) {
	ci.Parallel(t)

	verbs := []string{http.MethodGet, http.MethodPost, http.MethodPut}
	path := "schedulers"

	for _, verb := range verbs {
		t.Run(verb, func(t *testing.T) {
			httpTest(t, nil, func(s *TestAgent) {
				s.Agent.server = nil
				req, err := http.NewRequest(verb, fmt.Sprintf("/v1/agent/%v", path), nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				_, err = s.Server.AgentSchedulerWorkerInfoRequest(respW, req)

				require.Error(t, err)
				codedErr, ok := err.(HTTPCodedError)
				require.True(t, ok, "expected a HTTPCodedError")
				require.Equal(t, http.StatusBadRequest, codedErr.Code())
				require.Equal(t, ErrServerOnly, codedErr.Error())
			})
		})
	}
}

func TestHTTP_AgentSchedulerWorkerConfigRequest_Client(t *testing.T) {
	ci.Parallel(t)

	verbs := []string{http.MethodGet, http.MethodPost, http.MethodPut}
	path := "schedulers/config"

	for _, verb := range verbs {
		t.Run(verb, func(t *testing.T) {
			httpTest(t, nil, func(s *TestAgent) {
				s.Agent.server = nil
				req, err := http.NewRequest(verb, fmt.Sprintf("/v1/agent/%v", path), nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				_, err = s.Server.AgentSchedulerWorkerInfoRequest(respW, req)

				require.Error(t, err)
				codedErr, ok := err.(HTTPCodedError)
				require.True(t, ok, "expected a HTTPCodedError")
				require.Equal(t, http.StatusBadRequest, codedErr.Code())
				require.Equal(t, ErrServerOnly, codedErr.Error())
			})
		})
	}
}
