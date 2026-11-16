// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func promSDTestAlloc() *structs.Allocation {
	alloc := mock.Alloc()
	alloc.Name = alloc.JobID + ".web[3]"
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.Job.Meta = map[string]string{"user-id": "github|abc"}
	alloc.Job.LookupTaskGroup(alloc.TaskGroup).Meta = map[string]string{"app_id": "kling"}
	alloc.AllocatedResources.Shared.Ports = structs.AllocatedPorts{
		{Label: "http", Value: 20001, To: 8080, HostIP: "10.0.0.5"},
		{Label: "metrics", Value: 20002, To: 9090, HostIP: "10.0.0.5"},
	}
	return alloc
}

func TestAllocPromSDTargetGroups(t *testing.T) {
	ci.Parallel(t)

	nodeLabels := map[string]string{"__meta_nomad_node_id": "node-1"}
	alloc := promSDTestAlloc()

	groups := allocPromSDTargetGroups(alloc, nodeLabels, "", testlog.HCLogger(t))
	require.Len(t, groups, 2)

	byPort := map[string]*PromSDTargetGroup{}
	for _, g := range groups {
		byPort[g.Labels["__meta_nomad_port_label"]] = g
	}

	metrics := byPort["metrics"]
	require.NotNil(t, metrics)
	require.Equal(t, []string{"10.0.0.5:20002"}, metrics.Targets)
	require.Equal(t, "node-1", metrics.Labels["__meta_nomad_node_id"])
	require.Equal(t, alloc.ID, metrics.Labels["__meta_nomad_alloc_id"])
	require.Equal(t, alloc.JobID, metrics.Labels["__meta_nomad_job_id"])
	require.Equal(t, alloc.Namespace, metrics.Labels["__meta_nomad_namespace"])
	require.Equal(t, alloc.TaskGroup, metrics.Labels["__meta_nomad_task_group"])
	require.Equal(t, "3", metrics.Labels["__meta_nomad_alloc_index"])
	require.Equal(t, "10.0.0.5", metrics.Labels["__meta_nomad_address"])
	require.Equal(t, "20002", metrics.Labels["__meta_nomad_port"])

	// Meta keys are sanitized for Prometheus label name rules; group meta
	// is exposed alongside job meta.
	require.Equal(t, "github|abc", metrics.Labels["__meta_nomad_meta_user_id"])
	require.Equal(t, "kling", metrics.Labels["__meta_nomad_meta_app_id"])

	httpGroup := byPort["http"]
	require.NotNil(t, httpGroup)
	require.Equal(t, []string{"10.0.0.5:20001"}, httpGroup.Targets)
}

func TestAllocPromSDTargetGroups_PortFilter(t *testing.T) {
	ci.Parallel(t)

	alloc := promSDTestAlloc()

	groups := allocPromSDTargetGroups(alloc, nil, "metrics", testlog.HCLogger(t))
	require.Len(t, groups, 1)
	require.Equal(t, []string{"10.0.0.5:20002"}, groups[0].Targets)

	groups = allocPromSDTargetGroups(alloc, nil, "nope", testlog.HCLogger(t))
	require.Empty(t, groups)
}

func TestAllocPromSDTargetGroups_LegacyTaskNetworks(t *testing.T) {
	ci.Parallel(t)

	alloc := promSDTestAlloc()
	alloc.AllocatedResources.Shared.Ports = nil
	alloc.AllocatedResources.Tasks = map[string]*structs.AllocatedTaskResources{
		"web": {
			Networks: []*structs.NetworkResource{
				{
					IP:            "192.168.0.100",
					DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
					ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
				},
			},
		},
	}

	groups := allocPromSDTargetGroups(alloc, nil, "", testlog.HCLogger(t))
	require.Len(t, groups, 2)

	byPort := map[string]*PromSDTargetGroup{}
	for _, g := range groups {
		byPort[g.Labels["__meta_nomad_port_label"]] = g
	}
	require.Equal(t, []string{"192.168.0.100:9876"}, byPort["http"].Targets)
	require.Equal(t, []string{"192.168.0.100:5000"}, byPort["admin"].Targets)
}

func TestAllocPromSDTargetGroups_SkipsIncomplete(t *testing.T) {
	ci.Parallel(t)

	// Ports without a bound host IP or value cannot be scraped.
	alloc := promSDTestAlloc()
	alloc.AllocatedResources.Shared.Ports = structs.AllocatedPorts{
		{Label: "metrics", Value: 0, HostIP: "10.0.0.5"},
		{Label: "http", Value: 20001, HostIP: ""},
	}
	require.Empty(t, allocPromSDTargetGroups(alloc, nil, "", testlog.HCLogger(t)))

	// The same holds when the broken port is explicitly selected.
	require.Empty(t, allocPromSDTargetGroups(alloc, nil, "http", testlog.HCLogger(t)))
}

func TestAllocPromSDTargetGroups_MetaPrecedence(t *testing.T) {
	ci.Parallel(t)

	// Distinct keys that sanitize to the same label name resolve to a
	// deterministic winner: keys are emitted in sorted order, so the last
	// sorted key wins ("user_id" sorts after "user-id").
	alloc := promSDTestAlloc()
	alloc.Job.Meta = map[string]string{
		"user-id": "from-dash",
		"user_id": "from-underscore",
	}
	for range 5 {
		groups := allocPromSDTargetGroups(alloc, nil, "metrics", testlog.HCLogger(t))
		require.Len(t, groups, 1)
		require.Equal(t, "from-underscore", groups[0].Labels["__meta_nomad_meta_user_id"])
	}

	// Task group meta overrides job meta for the same key.
	alloc = promSDTestAlloc()
	alloc.Job.Meta = map[string]string{"app_id": "job-level"}
	alloc.Job.LookupTaskGroup(alloc.TaskGroup).Meta = map[string]string{"app_id": "group-level"}
	groups := allocPromSDTargetGroups(alloc, nil, "metrics", testlog.HCLogger(t))
	require.Len(t, groups, 1)
	require.Equal(t, "group-level", groups[0].Labels["__meta_nomad_meta_app_id"])
}

func TestAllocPromSDTargetGroups_MissingTaskGroup(t *testing.T) {
	ci.Parallel(t)

	// An allocation whose task group cannot be found in its job still
	// yields targets; only the group meta labels are absent.
	alloc := promSDTestAlloc()
	alloc.TaskGroup = "does-not-exist"

	groups := allocPromSDTargetGroups(alloc, nil, "", testlog.HCLogger(t))
	require.Len(t, groups, 2)
	for _, g := range groups {
		require.Equal(t, "github|abc", g.Labels["__meta_nomad_meta_user_id"])
		require.NotContains(t, g.Labels, "__meta_nomad_meta_app_id")
	}
}

func TestPromSDTargetGroupsForAllocs_StatusFilter(t *testing.T) {
	ci.Parallel(t)

	running := promSDTestAlloc()
	pending := promSDTestAlloc()
	pending.ClientStatus = structs.AllocClientStatusPending
	complete := promSDTestAlloc()
	complete.ClientStatus = structs.AllocClientStatusComplete
	failed := promSDTestAlloc()
	failed.ClientStatus = structs.AllocClientStatusFailed

	allocs := []*structs.Allocation{pending, running, complete, failed}
	groups := promSDTargetGroupsForAllocs(allocs, nil, "", testlog.HCLogger(t))
	require.Len(t, groups, 2)
	for _, g := range groups {
		require.Equal(t, running.ID, g.Labels["__meta_nomad_alloc_id"])
	}
}

func TestPromSDTargetGroupsForAllocs_IncompleteRunningAlloc(t *testing.T) {
	ci.Parallel(t)

	// A running allocation with corrupted state (nil Job or resources)
	// is skipped without affecting healthy allocations and without
	// panicking.
	healthy := promSDTestAlloc()
	noJob := promSDTestAlloc()
	noJob.Job = nil
	noResources := promSDTestAlloc()
	noResources.AllocatedResources = nil

	allocs := []*structs.Allocation{noJob, healthy, noResources}
	groups := promSDTargetGroupsForAllocs(allocs, nil, "", testlog.HCLogger(t))
	require.Len(t, groups, 2)
	for _, g := range groups {
		require.Equal(t, healthy.ID, g.Labels["__meta_nomad_alloc_id"])
	}
}

func TestPromSDTargetGroupsForAllocs_Sort(t *testing.T) {
	ci.Parallel(t)

	a := promSDTestAlloc()
	a.ID = "aaaaaaaa-0000-0000-0000-000000000000"
	b := promSDTestAlloc()
	b.ID = "bbbbbbbb-0000-0000-0000-000000000000"

	// Feed in reverse order; output must be sorted by alloc ID then port
	// label regardless of input order.
	groups := promSDTargetGroupsForAllocs([]*structs.Allocation{b, a}, nil, "", testlog.HCLogger(t))
	require.Len(t, groups, 4)

	var order []string
	for _, g := range groups {
		order = append(order, g.Labels["__meta_nomad_alloc_id"][:8]+"/"+g.Labels["__meta_nomad_port_label"])
	}
	require.Equal(t, []string{"aaaaaaaa/http", "aaaaaaaa/metrics", "bbbbbbbb/http", "bbbbbbbb/metrics"}, order)
}

func TestPromSDTargetGroups_EmptyIsListNotNull(t *testing.T) {
	ci.Parallel(t)

	// The HTTP SD contract requires a JSON list; null is a parse error in
	// Prometheus. Empty input must produce a non-nil empty slice.
	groups := promSDTargetGroupsForAllocs(nil, nil, "", testlog.HCLogger(t))
	require.NotNil(t, groups)
	require.Empty(t, groups)
}

func TestPromSDTargetGroup_WireFormat(t *testing.T) {
	ci.Parallel(t)

	// The agent serializes responses with go-codec, which honors the json
	// struct tags as a fallback. Prometheus requires lowercase
	// "targets"/"labels" keys; lock the wire format in.
	group := &PromSDTargetGroup{
		Targets: []string{"10.0.0.5:20002"},
		Labels:  map[string]string{"__meta_nomad_port_label": "metrics"},
	}
	var buf bytes.Buffer
	require.NoError(t, codec.NewEncoder(&buf, structs.JsonHandleWithExtensions).Encode(group))
	out := buf.String()
	require.Contains(t, out, `"targets"`)
	require.Contains(t, out, `"labels"`)
	require.NotContains(t, out, `"Targets"`)
	require.NotContains(t, out, `"Labels"`)
}

func TestClientAllocPromSDRequest(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Wrong method is rejected.
		req, err := http.NewRequest(http.MethodPost, "/v1/client/allocations/prometheus-sd", nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()
		_, err = s.Server.ClientAllocPromSDRequest(respW, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), ErrInvalidMethod)

		// node_id is rejected: this endpoint serves local state only.
		req, err = http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd?node_id=some-node", nil)
		require.NoError(t, err)
		respW = httptest.NewRecorder()
		_, err = s.Server.ClientAllocPromSDRequest(respW, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "node_id is not supported")

		// GET on a node with no allocations returns an empty list.
		req, err = http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd", nil)
		require.NoError(t, err)
		respW = httptest.NewRecorder()
		obj, err := s.Server.ClientAllocPromSDRequest(respW, req)
		require.NoError(t, err)
		groups, ok := obj.([]*PromSDTargetGroup)
		require.True(t, ok)
		require.Empty(t, groups)
	})
}

func TestClientAllocPromSDRequest_WireFormat(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Drive the full mux + wrap path so status code, content type,
		// and the empty-body shape are asserted on the actual bytes
		// Prometheus would receive.
		req, err := http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd", nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()
		s.Server.mux.ServeHTTP(respW, req)

		require.Equal(t, http.StatusOK, respW.Code)
		require.Equal(t, "application/json", respW.Result().Header.Get("Content-Type"))
		require.Equal(t, "[]", strings.TrimSpace(respW.Body.String()))
	})
}

func TestClientAllocPromSDRequest_NoClient(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		c := s.client
		s.client = nil
		defer func() { s.client = c }()

		req, err := http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd", nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()
		_, err = s.Server.ClientAllocPromSDRequest(respW, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not running a Nomad Client")
	})
}

func TestClientAllocPromSDRequest_ACL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Without a token the request is denied.
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd", nil)
			require.NoError(t, err)
			respW := httptest.NewRecorder()
			_, err = s.Server.ClientAllocPromSDRequest(respW, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), structs.ErrPermissionDenied.Error())
		}

		// A token without node:read is denied.
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd", nil)
			require.NoError(t, err)
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
			setToken(req, token)
			respW := httptest.NewRecorder()
			_, err = s.Server.ClientAllocPromSDRequest(respW, req)
			require.Error(t, err)
			require.Equal(t, structs.ErrPermissionDenied.Error(), err.Error())
		}

		// A token with node:read is allowed.
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd", nil)
			require.NoError(t, err)
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			respW := httptest.NewRecorder()
			obj, err := s.Server.ClientAllocPromSDRequest(respW, req)
			require.NoError(t, err)
			require.NotNil(t, obj)
		}

		// A management token is allowed.
		{
			req, err := http.NewRequest(http.MethodGet, "/v1/client/allocations/prometheus-sd", nil)
			require.NoError(t, err)
			setToken(req, s.RootToken)
			respW := httptest.NewRecorder()
			obj, err := s.Server.ClientAllocPromSDRequest(respW, req)
			require.NoError(t, err)
			require.NotNil(t, obj)
		}
	})
}
