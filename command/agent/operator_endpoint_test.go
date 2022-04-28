package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestOperator_SchedulerGetConfiguration(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		require := require.New(t)
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/operator/scheduler/configuration", body)
		resp := httptest.NewRecorder()
		obj, err := s.Server.OperatorSchedulerConfiguration(resp, req)
		require.Nil(err)
		require.Equal(200, resp.Code)
		out, ok := obj.(structs.SchedulerConfigurationResponse)
		require.True(ok)

		// Only system jobs can preempt other jobs by default.
		require.True(out.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
		require.False(out.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
		require.False(out.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
		require.False(out.SchedulerConfig.MemoryOversubscriptionEnabled)
	})
}

func TestOperator_SchedulerSetConfiguration(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		require := require.New(t)
		body := bytes.NewBuffer([]byte(`
{
  "MemoryOversubscriptionEnabled": true,
  "PreemptionConfig": {
    "SystemSchedulerEnabled": true,
    "ServiceSchedulerEnabled": true
  }
}`))
		req, _ := http.NewRequest("PUT", "/v1/operator/scheduler/configuration", body)
		resp := httptest.NewRecorder()
		setResp, err := s.Server.OperatorSchedulerConfiguration(resp, req)
		require.Nil(err)
		require.Equal(200, resp.Code)
		schedSetResp, ok := setResp.(structs.SchedulerSetConfigurationResponse)
		require.True(ok)
		require.NotZero(schedSetResp.Index)

		args := structs.GenericRequest{
			QueryOptions: structs.QueryOptions{
				Region: s.Config.Region,
			},
		}

		var reply structs.SchedulerConfigurationResponse
		err = s.RPC("Operator.SchedulerGetConfiguration", &args, &reply)
		require.Nil(err)
		require.True(reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
		require.True(reply.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
		require.True(reply.SchedulerConfig.MemoryOversubscriptionEnabled)
	})
}

func TestOperator_SchedulerCASConfiguration(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		require := require.New(t)
		body := bytes.NewBuffer([]byte(`{"PreemptionConfig": {
                     "SystemSchedulerEnabled": true,
                     "BatchSchedulerEnabled":true
        }}`))
		req, _ := http.NewRequest("PUT", "/v1/operator/scheduler/configuration", body)
		resp := httptest.NewRecorder()
		setResp, err := s.Server.OperatorSchedulerConfiguration(resp, req)
		require.Nil(err)
		require.Equal(200, resp.Code)
		schedSetResp, ok := setResp.(structs.SchedulerSetConfigurationResponse)
		require.True(ok)
		require.NotZero(schedSetResp.Index)

		args := structs.GenericRequest{
			QueryOptions: structs.QueryOptions{
				Region: s.Config.Region,
			},
		}

		var reply structs.SchedulerConfigurationResponse
		if err := s.RPC("Operator.SchedulerGetConfiguration", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		require.True(reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
		require.True(reply.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)

		// Create a CAS request, bad index
		{
			buf := bytes.NewBuffer([]byte(`{"PreemptionConfig": {
                     "SystemSchedulerEnabled": false,
                     "BatchSchedulerEnabled":true
        }}`))
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/operator/scheduler/configuration?cas=%d", reply.QueryMeta.Index-1), buf)
			resp := httptest.NewRecorder()
			setResp, err := s.Server.OperatorSchedulerConfiguration(resp, req)
			require.Nil(err)
			// Verify that the response has Updated=false
			schedSetResp, ok := setResp.(structs.SchedulerSetConfigurationResponse)
			require.True(ok)
			require.NotZero(schedSetResp.Index)
			require.False(schedSetResp.Updated)
		}

		// Create a CAS request, good index
		{
			buf := bytes.NewBuffer([]byte(`{"PreemptionConfig": {
                     "SystemSchedulerEnabled": false,
                     "BatchSchedulerEnabled":false
        }}`))
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/operator/scheduler/configuration?cas=%d", reply.QueryMeta.Index), buf)
			resp := httptest.NewRecorder()
			setResp, err := s.Server.OperatorSchedulerConfiguration(resp, req)
			require.Nil(err)
			// Verify that the response has Updated=true
			schedSetResp, ok := setResp.(structs.SchedulerSetConfigurationResponse)
			require.True(ok)
			require.NotZero(schedSetResp.Index)
			require.True(schedSetResp.Updated)
		}

		// Verify the update
		if err := s.RPC("Operator.SchedulerGetConfiguration", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		require.False(reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
		require.False(reply.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	})
}

func TestOperator_SnapshotRequests(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "nomadtest-operator-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	snapshotPath := filepath.Join(dir, "snapshot.bin")
	job := mock.Job()

	// test snapshot generation
	httpTest(t, func(c *Config) {
		c.Server.BootstrapExpect = 1
		c.DevMode = false
		c.DataDir = path.Join(dir, "server")
		c.AdvertiseAddrs.HTTP = "127.0.0.1"
		c.AdvertiseAddrs.RPC = "127.0.0.1"
		c.AdvertiseAddrs.Serf = "127.0.0.1"

		// don't actually run the job
		c.Client.Enabled = false
	}, func(s *TestAgent) {
		// make a simple update
		jargs := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var jresp structs.JobRegisterResponse
		err := s.Agent.RPC("Job.Register", &jargs, &jresp)
		require.NoError(t, err)

		// now actually snapshot
		req, _ := http.NewRequest("GET", "/v1/operator/snapshot", nil)
		resp := httptest.NewRecorder()
		_, err = s.Server.SnapshotRequest(resp, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Code)

		digest := resp.Header().Get("Digest")
		require.NotEmpty(t, digest)
		require.Contains(t, digest, "sha-256=")

		hash := sha256.New()
		f, err := os.Create(snapshotPath)
		require.NoError(t, err)
		defer f.Close()

		_, err = io.Copy(io.MultiWriter(f, hash), resp.Body)
		require.NoError(t, err)

		expectedChecksum := "sha-256=" + base64.StdEncoding.EncodeToString(hash.Sum(nil))
		require.Equal(t, digest, expectedChecksum)
	})

	// test snapshot restoration
	httpTest(t, func(c *Config) {
		c.Server.BootstrapExpect = 1
		c.DevMode = false
		c.DataDir = path.Join(dir, "server2")
		c.AdvertiseAddrs.HTTP = "127.0.0.1"
		c.AdvertiseAddrs.RPC = "127.0.0.1"
		c.AdvertiseAddrs.Serf = "127.0.0.1"

		// don't actually run the job
		c.Client.Enabled = false
	}, func(s *TestAgent) {
		jobExists := func() bool {
			// check job isn't present
			req, _ := http.NewRequest("GET", "/v1/job/"+job.ID, nil)
			resp := httptest.NewRecorder()
			j, _ := s.Server.jobCRUD(resp, req, job.ID)
			return j != nil
		}

		// job doesn't get initially
		require.False(t, jobExists())

		// restrore and check if job exists after
		f, err := os.Open(snapshotPath)
		require.NoError(t, err)
		defer f.Close()

		req, _ := http.NewRequest("PUT", "/v1/operator/snapshot", f)
		resp := httptest.NewRecorder()
		_, err = s.Server.SnapshotRequest(resp, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Code)

		require.True(t, jobExists())
	})

}
