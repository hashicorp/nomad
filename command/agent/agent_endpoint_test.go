package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestHTTP_AgentSelf(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/agent/self", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AgentSelfRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the job
		self := obj.(agentSelf)
		if self.Config == nil {
			t.Fatalf("bad: %#v", self)
		}
		if len(self.Stats) == 0 {
			t.Fatalf("bad: %#v", self)
		}

		// Check the Vault config
		if self.Config.Vault.Token != "" {
			t.Fatalf("bad: %#v", self)
		}

		// Assign a Vault token and require it is redacted.
		s.Config.Vault.Token = "badc0deb-adc0-deba-dc0d-ebadc0debadc"
		respW = httptest.NewRecorder()
		obj, err = s.Server.AgentSelfRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		self = obj.(agentSelf)
		if self.Config.Vault.Token != "<redacted>" {
			t.Fatalf("bad: %#v", self)
		}
	})
}

func TestHTTP_AgentSelf_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/agent/self", nil)
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
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Determine the join address
		member := s.Agent.Server().LocalMember()
		addr := fmt.Sprintf("%s:%d", member.Addr, member.Port)

		// Make the HTTP request
		req, err := http.NewRequest("PUT",
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
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/agent/members", nil)
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
	t.Parallel()
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/agent/members", nil)
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

func TestHTTP_AgentForceLeave(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/agent/force-leave?node=foo", nil)
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
	t.Parallel()
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/agent/force-leave?node=foo", nil)
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
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
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

		// Create the request
		req, err := http.NewRequest("PUT", "/v1/agent/servers", nil)
		require.Nil(err)

		// Send the request
		respW := httptest.NewRecorder()
		_, err = s.Server.AgentServersRequest(respW, req)
		require.NotNil(err)
		require.Contains(err.Error(), "missing server address")

		// Create a valid request
		req, err = http.NewRequest("PUT", "/v1/agent/servers?address=127.0.0.1%3A4647&address=127.0.0.2%3A4647&address=127.0.0.3%3A4647", nil)
		require.Nil(err)

		// Send the request which should fail
		respW = httptest.NewRecorder()
		_, err = s.Server.AgentServersRequest(respW, req)
		require.NotNil(err)

		// Retrieve the servers again
		req, err = http.NewRequest("GET", "/v1/agent/servers", nil)
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
	t.Parallel()
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
		req, err := http.NewRequest("PUT", path, nil)
		require.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.AgentServersRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
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
	t.Parallel()
	require := require.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Create list request
		req, err := http.NewRequest("GET", "/v1/agent/servers", nil)
		require.Nil(err)

		expected := []string{
			s.GetConfig().AdvertiseAddrs.RPC,
		}

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.AgentServersRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
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
	t.Parallel()

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="

	httpTest(t, func(c *Config) {
		c.Server.EncryptKey = key1
	}, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/agent/keyring/list", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW := httptest.NewRecorder()

		out, err := s.Server.KeyringOperationRequest(respW, req)
		require.Nil(t, err)
		kresp := out.(structs.KeyringResponse)
		require.Len(t, kresp.Keys, 1)
	})
}

func TestHTTP_AgentListKeys_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="

	cb := func(c *Config) {
		c.Server.EncryptKey = key1
	}

	httpACLTest(t, cb, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/agent/keyring/list", nil)
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
	t.Parallel()

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="
	key2 := "wH1Bn9hlJ0emgWB1JttVRA=="

	httpTest(t, func(c *Config) {
		c.Server.EncryptKey = key1
	}, func(s *TestAgent) {
		b, err := json.Marshal(&structs.KeyringRequest{Key: key2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		req, err := http.NewRequest("GET", "/v1/agent/keyring/install", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.KeyringOperationRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		req, err = http.NewRequest("GET", "/v1/agent/keyring/list", bytes.NewReader(b))
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
	t.Parallel()

	key1 := "HS5lJ+XuTlYKWaeGYyG+/A=="
	key2 := "wH1Bn9hlJ0emgWB1JttVRA=="

	httpTest(t, func(c *Config) {
		c.Server.EncryptKey = key1
	}, func(s *TestAgent) {
		b, err := json.Marshal(&structs.KeyringRequest{Key: key2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		req, err := http.NewRequest("GET", "/v1/agent/keyring/install", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW := httptest.NewRecorder()
		_, err = s.Server.KeyringOperationRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		req, err = http.NewRequest("GET", "/v1/agent/keyring/remove", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW = httptest.NewRecorder()
		if _, err = s.Server.KeyringOperationRequest(respW, req); err != nil {
			t.Fatalf("err: %s", err)
		}

		req, err = http.NewRequest("GET", "/v1/agent/keyring/list", nil)
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
	t.Parallel()
	require := require.New(t)

	// Enable ACLs to ensure they're not enforced
	httpACLTest(t, nil, func(s *TestAgent) {
		// No ?type=
		{
			req, err := http.NewRequest("GET", "/v1/agent/health", nil)
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
			req, err := http.NewRequest("GET", "/v1/agent/health?type=client", nil)
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
			req, err := http.NewRequest("GET", "/v1/agent/health?type=server", nil)
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
			req, err := http.NewRequest("GET", "/v1/agent/health?type=client&type=server", nil)
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
	t.Parallel()
	require := require.New(t)

	// Enable ACLs to ensure they're not enforced
	httpACLTest(t, nil, func(s *TestAgent) {

		// Set s.Agent.server=nil to make server unhealthy if requested
		s.Agent.server = nil

		// No ?type= means server is just skipped
		{
			req, err := http.NewRequest("GET", "/v1/agent/health", nil)
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
			req, err := http.NewRequest("GET", "/v1/agent/health?type=server", nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			_, err = s.Server.HealthRequest(respW, req)
			require.NotNil(err)
			httpErr, ok := err.(HTTPCodedError)
			require.True(ok)
			require.Equal(500, httpErr.Code())
			require.Equal(`{"server":{"ok":false,"message":"server not enabled"}}`, err.Error())
		}
	})
}

func TestHTTP_AgentHealth_BadClient(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Enable ACLs to ensure they're not enforced
	httpACLTest(t, nil, func(s *TestAgent) {

		// Set s.Agent.client=nil to make server unhealthy if requested
		s.Agent.client = nil

		// No ?type= means client is just skipped
		{
			req, err := http.NewRequest("GET", "/v1/agent/health", nil)
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
			req, err := http.NewRequest("GET", "/v1/agent/health?type=client", nil)
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
