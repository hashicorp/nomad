package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
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

		// Assign a Vault token and assert it is redacted.
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

func TestHTTP_AgentJoin(t *testing.T) {
	// TODO(alexdadgar)
	// t.Parallel()
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

func TestHTTP_AgentSetServers(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Establish a baseline number of servers
		req, err := http.NewRequest("GET", "/v1/agent/servers", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW := httptest.NewRecorder()

		// Create the request
		req, err = http.NewRequest("PUT", "/v1/agent/servers", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Send the request
		respW = httptest.NewRecorder()
		_, err = s.Server.AgentServersRequest(respW, req)
		if err == nil || !strings.Contains(err.Error(), "missing server address") {
			t.Fatalf("expected missing servers error, got: %#v", err)
		}

		// Create a valid request
		req, err = http.NewRequest("PUT", "/v1/agent/servers?address=127.0.0.1%3A4647&address=127.0.0.2%3A4647&address=127.0.0.3%3A4647", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Send the request
		respW = httptest.NewRecorder()
		_, err = s.Server.AgentServersRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Retrieve the servers again
		req, err = http.NewRequest("GET", "/v1/agent/servers", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		respW = httptest.NewRecorder()

		// Make the request and check the result
		expected := map[string]bool{
			"127.0.0.1:4647": true,
			"127.0.0.2:4647": true,
			"127.0.0.3:4647": true,
		}
		out, err := s.Server.AgentServersRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		servers := out.([]string)
		if n := len(servers); n != len(expected) {
			t.Fatalf("expected %d servers, got: %d: %v", len(expected), n, servers)
		}
		received := make(map[string]bool, len(servers))
		for _, server := range servers {
			received[server] = true
		}
		foundCount := 0
		for k, _ := range received {
			_, found := expected[k]
			if found {
				foundCount++
			}
		}
		if foundCount != len(expected) {
			t.Fatalf("bad servers result")
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
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		kresp := out.(structs.KeyringResponse)
		if len(kresp.Keys) != 1 {
			t.Fatalf("bad: %v", kresp)
		}
	})
}

func TestHTTP_AgentInstallKey(t *testing.T) {
	// TODO(alexdadgar)
	// t.Parallel()

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
	// TODO(alexdadgar)
	// t.Parallel()

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
