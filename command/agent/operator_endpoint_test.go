package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_OperatorRaftConfiguration(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest("GET", "/v1/operator/raft/configuration", body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		obj, err := s.Server.OperatorRaftConfiguration(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		out, ok := obj.(structs.RaftConfigurationResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if len(out.Servers) != 1 ||
			!out.Servers[0].Leader ||
			!out.Servers[0].Voter {
			t.Fatalf("bad: %v", out)
		}
	})
}

func TestHTTP_OperatorRaftPeer(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest("DELETE", "/v1/operator/raft/peer?address=nope", body)
		assert.Nil(err)

		// If we get this error, it proves we sent the address all the
		// way through.
		resp := httptest.NewRecorder()
		_, err = s.Server.OperatorRaftPeer(resp, req)
		if err == nil || !strings.Contains(err.Error(),
			"address \"nope\" was not found in the Raft configuration") {
			t.Fatalf("err: %v", err)
		}
	})

	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest("DELETE", "/v1/operator/raft/peer?id=nope", body)
		assert.Nil(err)

		// If we get this error, it proves we sent the address all the
		// way through.
		resp := httptest.NewRecorder()
		_, err = s.Server.OperatorRaftPeer(resp, req)
		if err == nil || !strings.Contains(err.Error(),
			"id \"nope\" was not found in the Raft configuration") {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestOperator_AutopilotGetConfiguration(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/operator/autopilot/configuration", body)
		resp := httptest.NewRecorder()
		obj, err := s.Server.OperatorAutopilotConfiguration(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		out, ok := obj.(api.AutopilotConfiguration)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if !out.CleanupDeadServers {
			t.Fatalf("bad: %#v", out)
		}
	})
}

func TestOperator_AutopilotSetConfiguration(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer([]byte(`{"CleanupDeadServers": false}`))
		req, _ := http.NewRequest("PUT", "/v1/operator/autopilot/configuration", body)
		resp := httptest.NewRecorder()
		if _, err := s.Server.OperatorAutopilotConfiguration(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d, %q", resp.Code, resp.Body.String())
		}

		args := structs.GenericRequest{
			QueryOptions: structs.QueryOptions{
				Region: s.Config.Region,
			},
		}

		var reply structs.AutopilotConfig
		if err := s.RPC("Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if reply.CleanupDeadServers {
			t.Fatalf("bad: %#v", reply)
		}
	})
}

func TestOperator_AutopilotCASConfiguration(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer([]byte(`{"CleanupDeadServers": false}`))
		req, _ := http.NewRequest("PUT", "/v1/operator/autopilot/configuration", body)
		resp := httptest.NewRecorder()
		if _, err := s.Server.OperatorAutopilotConfiguration(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}

		args := structs.GenericRequest{
			QueryOptions: structs.QueryOptions{
				Region: s.Config.Region,
			},
		}

		var reply structs.AutopilotConfig
		if err := s.RPC("Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if reply.CleanupDeadServers {
			t.Fatalf("bad: %#v", reply)
		}

		// Create a CAS request, bad index
		{
			buf := bytes.NewBuffer([]byte(`{"CleanupDeadServers": true}`))
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/operator/autopilot/configuration?cas=%d", reply.ModifyIndex-1), buf)
			resp := httptest.NewRecorder()
			obj, err := s.Server.OperatorAutopilotConfiguration(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			if res := obj.(bool); res {
				t.Fatalf("should NOT work")
			}
		}

		// Create a CAS request, good index
		{
			buf := bytes.NewBuffer([]byte(`{"CleanupDeadServers": true}`))
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/operator/autopilot/configuration?cas=%d", reply.ModifyIndex), buf)
			resp := httptest.NewRecorder()
			obj, err := s.Server.OperatorAutopilotConfiguration(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			if res := obj.(bool); !res {
				t.Fatalf("should work")
			}
		}

		// Verify the update
		if err := s.RPC("Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if !reply.CleanupDeadServers {
			t.Fatalf("bad: %#v", reply)
		}
	})
}

func TestOperator_ServerHealth(t *testing.T) {
	httpTest(t, func(c *Config) {
		c.Server.RaftProtocol = 3
	}, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/operator/autopilot/health", body)
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := s.Server.OperatorServerHealth(resp, req)
			if err != nil {
				r.Fatalf("err: %v", err)
			}
			if resp.Code != 200 {
				r.Fatalf("bad code: %d", resp.Code)
			}
			out, ok := obj.(*api.OperatorHealthReply)
			if !ok {
				r.Fatalf("unexpected: %T", obj)
			}
			if len(out.Servers) != 1 ||
				!out.Servers[0].Healthy ||
				out.Servers[0].Name != s.server.LocalMember().Name ||
				out.Servers[0].SerfStatus != "alive" ||
				out.FailureTolerance != 0 {
				r.Fatalf("bad: %v, %q", out, s.server.LocalMember().Name)
			}
		})
	})
}

func TestOperator_ServerHealth_Unhealthy(t *testing.T) {
	t.Parallel()
	httpTest(t, func(c *Config) {
		c.Server.RaftProtocol = 3
		c.Autopilot.LastContactThreshold = -1 * time.Second
	}, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/operator/autopilot/health", body)
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := s.Server.OperatorServerHealth(resp, req)
			if err != nil {
				r.Fatalf("err: %v", err)
			}
			if resp.Code != 429 {
				r.Fatalf("bad code: %d, %v", resp.Code, obj.(*api.OperatorHealthReply))
			}
			out, ok := obj.(*api.OperatorHealthReply)
			if !ok {
				r.Fatalf("unexpected: %T", obj)
			}
			if len(out.Servers) != 1 ||
				out.Healthy ||
				out.Servers[0].Name != s.server.LocalMember().Name {
				r.Fatalf("bad: %#v", out.Servers)
			}
		})
	})
}
