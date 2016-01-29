package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTP_AgentSelf(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
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
	})
}

func TestHTTP_AgentJoin(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
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
	httpTest(t, nil, func(s *TestServer) {
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
		members := obj.([]Member)
		if len(members) != 1 {
			t.Fatalf("bad: %#v", members)
		}
	})
}

func TestHTTP_AgentForceLeave(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
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
	httpTest(t, nil, func(s *TestServer) {
		// Create the request
		req, err := http.NewRequest("PUT", "/v1/agent/servers", nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Send the request
		respW := httptest.NewRecorder()
		_, err = s.Server.AgentServersRequest(respW, req)
		if err == nil || !strings.Contains(err.Error(), "missing server address") {
			t.Fatalf("expected missing servers error, got: %#v", err)
		}

		// Create a valid request
		req, err = http.NewRequest("PUT", "/v1/agent/servers?address=foo&address=bar", nil)
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
		out, err := s.Server.AgentServersRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		servers := out.([]string)
		if n := len(servers); n != 2 {
			t.Fatalf("expected 2 servers, got: %d", n)
		}
		if servers[0] != "foo:4647" || servers[1] != "bar:4647" {
			t.Fatalf("bad servers result: %v", servers)
		}
	})
}
