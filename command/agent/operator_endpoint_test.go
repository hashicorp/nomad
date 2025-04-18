// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/require"
)

func TestHTTP_OperatorRaftConfiguration(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest(http.MethodGet, "/v1/operator/raft/configuration", body)
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
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest(http.MethodDelete, "/v1/operator/raft/peer?id=nope", body)
		must.NoError(t, err)

		// If we get this error, it proves we sent the address all the
		// way through.
		resp := httptest.NewRecorder()
		_, err = s.Server.OperatorRaftPeer(resp, req)
		must.ErrorContains(t, err,
			"id \"nope\" was not found in the Raft configuration")
	})
}

func TestHTTP_OperatorRaftTransferLeadership(t *testing.T) {
	ci.Parallel(t)
	configCB := func(c *Config) {
		c.Client.Enabled = false
		c.Server.NumSchedulers = pointer.Of(0)
	}

	httpTest(t, configCB, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		badMethods := []string{
			http.MethodConnect,
			http.MethodDelete,
			http.MethodGet,
			http.MethodHead,
			http.MethodOptions,
			http.MethodPatch,
			http.MethodTrace,
		}
		for _, tc := range badMethods {
			tc := tc
			t.Run(tc+" method errors", func(t *testing.T) {
				req, err := http.NewRequest(tc, "/v1/operator/raft/transfer-leadership?address=nope", body)
				must.NoError(t, err)

				resp := httptest.NewRecorder()
				_, err = s.Server.OperatorRaftTransferLeadership(resp, req)

				must.Error(t, err)
				must.ErrorContains(t, err, "Invalid method")
				body.Reset()
			})
		}

		apiErrTCs := []struct {
			name     string
			qs       string
			expected string
		}{
			{
				name:     "URL with id and address errors",
				qs:       `?id=foo&address=bar`,
				expected: "must specify either id or address",
			},
			{
				name:     "URL without id and address errors",
				qs:       ``,
				expected: "must specify id or address",
			},
			{
				name:     "URL with multiple id errors",
				qs:       `?id=foo&id=bar`,
				expected: "must specify only one id",
			},
			{
				name:     "URL with multiple address errors",
				qs:       `?address=foo&address=bar`,
				expected: "must specify only one address",
			},
			{
				name:     "URL with an empty id errors",
				qs:       `?id`,
				expected: "id must be non-empty",
			},
			{
				name:     "URL with an empty address errors",
				qs:       `?address`,
				expected: "address must be non-empty",
			},
			{
				name:     "an invalid id errors",
				qs:       `?id=foo`,
				expected: "id must be a uuid",
			},
			{
				name:     "URL with an empty address errors",
				qs:       `?address=bar`,
				expected: "address must be in IP:port format",
			},
		}
		for _, tc := range apiErrTCs {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				req, err := http.NewRequest(
					http.MethodPut,
					"/v1/operator/raft/transfer-leadership"+tc.qs,
					body,
				)
				must.NoError(t, err)

				resp := httptest.NewRecorder()
				_, err = s.Server.OperatorRaftTransferLeadership(resp, req)

				must.Error(t, err)
				must.ErrorContains(t, err, tc.expected)
				body.Reset()
			})
		}
	})

	testID := uuid.Generate()
	apiOkTCs := []struct {
		name     string
		qs       string
		expected string
	}{
		{
			"id",
			"?id=" + testID,
			`id "` + testID + `" was not found in the Raft configuration`,
		},
		{
			"address",
			"?address=9.9.9.9:8000",
			`address "9.9.9.9:8000" was not found in the Raft configuration`,
		},
	}
	for _, tc := range apiOkTCs {
		tc := tc
		t.Run(tc.name+" can roundtrip", func(t *testing.T) {
			httpTest(t, configCB, func(s *TestAgent) {
				body := bytes.NewBuffer(nil)
				req, err := http.NewRequest(
					http.MethodPut,
					"/v1/operator/raft/transfer-leadership"+tc.qs,
					body,
				)
				must.NoError(t, err)

				// If we get this error, it proves we sent the parameter all the
				// way through.
				resp := httptest.NewRecorder()
				_, err = s.Server.OperatorRaftTransferLeadership(resp, req)
				must.ErrorContains(t, err, tc.expected)
			})
		})
	}
}

func TestOperator_AutopilotGetConfiguration(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest(http.MethodGet, "/v1/operator/autopilot/configuration", body)
		resp := httptest.NewRecorder()
		obj, err := s.Server.OperatorAutopilotConfiguration(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != http.StatusOK {
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
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer([]byte(`{"CleanupDeadServers": false}`))
		req, _ := http.NewRequest(http.MethodPut, "/v1/operator/autopilot/configuration", body)
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
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer([]byte(`{"CleanupDeadServers": false}`))
		req, _ := http.NewRequest(http.MethodPut, "/v1/operator/autopilot/configuration", body)
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
			req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/v1/operator/autopilot/configuration?cas=%d", reply.ModifyIndex-1), buf)
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
			req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/v1/operator/autopilot/configuration?cas=%d", reply.ModifyIndex), buf)
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
	ci.Parallel(t)

	httpTest(t, func(c *Config) {
		c.Server.RaftProtocol = 3
	}, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest(http.MethodGet, "/v1/operator/autopilot/health", body)
		f := func() error {
			resp := httptest.NewRecorder()
			obj, err := s.Server.OperatorServerHealth(resp, req)
			if err != nil {
				return fmt.Errorf("failed to get operator server health: %w", err)
			}
			if code := resp.Code; code != 200 {
				return fmt.Errorf("response code not 200, got: %d", code)
			}
			out := obj.(*api.OperatorHealthReply)
			if n := len(out.Servers); n != 1 {
				return fmt.Errorf("expected 1 server, got: %d", n)
			}
			s1, s2 := out.Servers[0].Name, s.server.LocalMember().Name
			if s1 != s2 {
				return fmt.Errorf("expected server names to match, got %s and %s", s1, s2)
			}
			if out.Servers[0].SerfStatus != "alive" {
				return fmt.Errorf("expected serf status to be alive, got: %s", out.Servers[0].SerfStatus)
			}
			if out.FailureTolerance != 0 {
				return fmt.Errorf("expected failure tolerance of 0, got: %d", out.FailureTolerance)
			}
			return nil
		}
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(f),
			wait.Timeout(10*time.Second),
			wait.Gap(1*time.Second),
		))
	})
}

func TestOperator_ServerHealth_Unhealthy(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, func(c *Config) {
		c.Server.RaftProtocol = 3
		c.Autopilot.LastContactThreshold = -1 * time.Second
	}, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest(http.MethodGet, "/v1/operator/autopilot/health", body)
		f := func() error {
			resp := httptest.NewRecorder()
			obj, err := s.Server.OperatorServerHealth(resp, req)
			if err != nil {
				return fmt.Errorf("failed to get operator server health: %w", err)
			}
			if code := resp.Code; code != 429 {
				return fmt.Errorf("expected code 429, got: %d", code)
			}
			out := obj.(*api.OperatorHealthReply)
			if n := len(out.Servers); n != 1 {
				return fmt.Errorf("expected 1 server, got: %d", n)
			}
			if out.Healthy {
				return fmt.Errorf("expected server to be unhealthy")
			}
			s1, s2 := out.Servers[0].Name, s.server.LocalMember().Name
			if s1 != s2 {
				return fmt.Errorf("expected server names to match, got %s and %s", s1, s2)
			}
			return nil
		}
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(f),
			wait.Timeout(10*time.Second),
			wait.Gap(1*time.Second),
		))
	})
}

func TestOperator_AutopilotHealth(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, func(c *Config) {
		c.Server.RaftProtocol = 3
	}, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest(http.MethodGet, "/v1/operator/autopilot/health", body)
		f := func() error {
			resp := httptest.NewRecorder()
			obj, err := s.Server.OperatorServerHealth(resp, req)
			if err != nil {
				return fmt.Errorf("failed to get operator server state: %w", err)
			}
			if code := resp.Code; code != 200 {
				return fmt.Errorf("response code not 200, got: %d", code)
			}
			out := obj.(*api.OperatorHealthReply)
			if n := len(out.Servers); n != 1 {
				return fmt.Errorf("expected 1 server, got: %d", n)
			}
			serfMember := s.server.LocalMember()
			id, ok := serfMember.Tags["id"]
			if !ok {
				t.Errorf("Tag not found")
			}
			var leader api.ServerHealth
			for _, srv := range out.Servers {
				if srv.ID == id {
					leader = srv
					break
				}
			}

			t.Log("serfMember", serfMember)
			s1, s2 := leader.ID, id
			if s1 != s2 {
				return fmt.Errorf("expected server names to match, got %s and %s", s1, s2)
			}
			if leader.Healthy != true {
				return fmt.Errorf("expected autopilot server status to be healthy, got: %t", leader.Healthy)
			}
			s1, s2 = out.Voters[0], id
			if s1 != s2 {
				return fmt.Errorf("expected server to be voter: %s", out.Voters[0])
			}
			return nil
		}
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(f),
			wait.Timeout(10*time.Second),
			wait.Gap(1*time.Second),
		))
	})
}

func TestOperator_SchedulerGetConfiguration(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest(http.MethodGet, "/v1/operator/scheduler/configuration", body)
		resp := httptest.NewRecorder()
		obj, err := s.Server.OperatorSchedulerConfiguration(resp, req)
		require.Nil(t, err)
		require.Equal(t, 200, resp.Code)
		out, ok := obj.(structs.SchedulerConfigurationResponse)
		require.True(t, ok)

		// Only system jobs can preempt other jobs by default.
		require.True(t, out.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
		require.False(t, out.SchedulerConfig.PreemptionConfig.SysBatchSchedulerEnabled)
		require.False(t, out.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
		require.False(t, out.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
		require.False(t, out.SchedulerConfig.MemoryOversubscriptionEnabled)
		require.False(t, out.SchedulerConfig.PauseEvalBroker)
	})
}

func TestOperator_SchedulerSetConfiguration(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		body := bytes.NewBuffer([]byte(`
{
  "MemoryOversubscriptionEnabled": true,
  "PauseEvalBroker": true,
  "PreemptionConfig": {
    "SystemSchedulerEnabled": true,
    "ServiceSchedulerEnabled": true
  }
}`))
		req, _ := http.NewRequest(http.MethodPut, "/v1/operator/scheduler/configuration", body)
		resp := httptest.NewRecorder()
		setResp, err := s.Server.OperatorSchedulerConfiguration(resp, req)
		require.Nil(t, err)
		require.Equal(t, 200, resp.Code)
		schedSetResp, ok := setResp.(structs.SchedulerSetConfigurationResponse)
		require.True(t, ok)
		require.NotZero(t, schedSetResp.Index)

		args := structs.GenericRequest{
			QueryOptions: structs.QueryOptions{
				Region: s.Config.Region,
			},
		}

		var reply structs.SchedulerConfigurationResponse
		err = s.RPC("Operator.SchedulerGetConfiguration", &args, &reply)
		require.Nil(t, err)
		require.True(t, reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
		require.False(t, reply.SchedulerConfig.PreemptionConfig.SysBatchSchedulerEnabled)
		require.False(t, reply.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
		require.True(t, reply.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
		require.True(t, reply.SchedulerConfig.MemoryOversubscriptionEnabled)
		require.True(t, reply.SchedulerConfig.PauseEvalBroker)
	})
}

func TestOperator_SchedulerCASConfiguration(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		require := require.New(t)
		body := bytes.NewBuffer([]byte(`{"PreemptionConfig": {
                     "SystemSchedulerEnabled": true,
                     "SysBatchSchedulerEnabled":true,
                     "BatchSchedulerEnabled":true
        }}`))
		req, _ := http.NewRequest(http.MethodPut, "/v1/operator/scheduler/configuration", body)
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
		require.True(reply.SchedulerConfig.PreemptionConfig.SysBatchSchedulerEnabled)
		require.True(reply.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
		require.False(reply.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)

		// Create a CAS request, bad index
		{
			buf := bytes.NewBuffer([]byte(`{"PreemptionConfig": {
                     "SystemSchedulerEnabled": false,
                     "BatchSchedulerEnabled":true
        }}`))
			req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/v1/operator/scheduler/configuration?cas=%d", reply.QueryMeta.Index-1), buf)
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
			req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/v1/operator/scheduler/configuration?cas=%d", reply.QueryMeta.Index), buf)
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
		require.False(reply.SchedulerConfig.PreemptionConfig.SysBatchSchedulerEnabled)
		require.False(reply.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
		require.False(reply.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
	})
}

func TestOperator_SnapshotRequests(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()

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
		req, _ := http.NewRequest(http.MethodGet, "/v1/operator/snapshot", nil)
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
			req, _ := http.NewRequest(http.MethodGet, "/v1/job/"+job.ID, nil)
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

		req, _ := http.NewRequest(http.MethodPut, "/v1/operator/snapshot", f)
		resp := httptest.NewRecorder()
		_, err = s.Server.SnapshotRequest(resp, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Code)

		require.True(t, jobExists())
	})
}

func TestOperator_UpgradeCheckRequest_VaultWorkloadIdentity(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, func(c *Config) {
		c.Vaults[0].Enabled = pointer.Of(true)
		c.Vaults[0].Name = "default"
	}, func(s *TestAgent) {
		// Create a test job with a Vault block but without an identity.
		job := mock.Job()
		job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
			Cluster: "default",
		}

		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.JobRegisterResponse
		err := s.Agent.RPC("Job.Register", &args, &resp)
		must.NoError(t, err)

		// Make HTTP request to retrieve
		req, err := http.NewRequest(http.MethodGet, "/v1/operator/upgrade-check/vault-workload-identity", nil)
		must.NoError(t, err)
		respW := httptest.NewRecorder()

		obj, err := s.Server.UpgradeCheckRequest(respW, req)
		must.NoError(t, err)
		must.NotEq(t, "", respW.Header().Get("X-Nomad-Index"))
		must.NotEq(t, "", respW.Header().Get("X-Nomad-LastContact"))
		must.Eq(t, "true", respW.Header().Get("X-Nomad-KnownLeader"))

		upgradeCheck := obj.(structs.UpgradeCheckVaultWorkloadIdentityResponse)
		must.Len(t, 1, upgradeCheck.JobsWithoutVaultIdentity)
		must.Len(t, 0, upgradeCheck.VaultTokens)
		must.Eq(t, job.ID, upgradeCheck.JobsWithoutVaultIdentity[0].ID)
	})
}
