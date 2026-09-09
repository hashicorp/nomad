// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"math"
	"net"
	"testing"

	"github.com/hashicorp/go-version"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/peers"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/serf/serf"
	"github.com/shoenig/test/must"
)

func TestMaximumDesiredAllocations(t *testing.T) {
	ci.Parallel(t)
	job := &structs.Job{TaskGroups: []*structs.TaskGroup{
		{Name: "a", Count: 2}, {Name: "b", Count: 5}, {Name: "c", Count: 3},
		{Name: "required", Count: 4},
	}}
	must.Eq(t, 14, maximumDesiredAllocations(job))
	selection := &structs.TaskGroupSelection{Name: "runtime", Count: 2, Groups: []string{"a", "b", "c"}}
	job.GroupSelections = []*structs.TaskGroupSelection{selection}
	must.Eq(t, 12, maximumDesiredAllocations(job))
	selection.Count = 1
	must.Eq(t, 9, maximumDesiredAllocations(job))
	selection.Count = 0
	must.Eq(t, 4, maximumDesiredAllocations(job))
	selection.Count = 2
	job.GroupSelections = append(job.GroupSelections, &structs.TaskGroupSelection{
		Name: "other", Count: 1, Groups: []string{"required"},
	})
	must.Eq(t, 12, maximumDesiredAllocations(job))
	job.TaskGroups[0].Count = math.MaxInt
	must.Eq(t, math.MaxInt, maximumDesiredAllocations(job))
}

func TestJobEndpoint_Scale_GroupSelectionLimits(t *testing.T) {
	ci.Parallel(t)
	srv, cleanup := TestServer(t, func(config *Config) { config.JobMaxCount = 10 })
	defer cleanup()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)

	job := mock.Job()
	job.TaskGroups[0].Count = 5
	other := job.TaskGroups[0].Copy()
	other.Name = "alternative"
	other.Count = 4
	job.TaskGroups = append(job.TaskGroups, other)
	job.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web", "alternative"}}}
	must.NoError(t, srv.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))

	request := &structs.JobScaleRequest{
		JobID:        job.ID,
		Target:       map[string]string{structs.ScalingTargetGroup: "alternative"},
		Count:        new(int64(8)),
		WriteRequest: structs.WriteRequest{Region: "global", Namespace: job.Namespace},
	}
	var response structs.JobRegisterResponse
	// 5 + 8 exceeds the limit, but only one of these groups is required.
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Scale", request, &response))
	request.Count = new(int64(11))
	must.ErrorContains(t, msgpackrpc.CallWithCodec(codec, "Job.Scale", request, &response), "11 > 10")
	request.Count = new(int64(0))
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Scale", request, &response))
	request.Target[structs.ScalingTargetGroup] = "web"
	must.ErrorContains(t, msgpackrpc.CallWithCodec(codec, "Job.Scale", request, &response), "exceeds its 0 task groups with a positive count")
}

func TestJobValidate_GroupSelectionServerVersions(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name, region, build string
		status              serf.MemberStatus
		wantError           bool
	}{
		{"upgraded", "global", "2.0.6", serf.StatusAlive, false},
		{"old alive", "global", "2.0.5", serf.StatusAlive, true},
		{"old failed", "global", "2.0.5", serf.StatusFailed, true},
		{"old departed", "global", "2.0.5", serf.StatusLeft, false},
		{"other region", "other", "2.0.5", serf.StatusAlive, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := peers.NewPeerCache("global")
			cache.UpdatePeerSet(&peers.Parts{
				Name: "local", Region: "global", Status: serf.StatusAlive,
				Build: *minVersionGroupSelections, Addr: &net.TCPAddr{Port: 1},
			})
			cache.UpdatePeerSet(&peers.Parts{
				Name: "peer", Region: tc.region, Status: tc.status,
				Build: *version.Must(version.NewVersion(tc.build)), Addr: &net.TCPAddr{Port: 2},
			})
			v := &jobValidate{srv: &Server{
				config: &Config{Region: "global", JobMaxPriority: 100},
				serf:   &serf.Serf{}, peersCache: cache,
			}}
			job := mock.Job()
			_, err := v.Validate(job)
			must.NoError(t, err)
			job.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web"}}}
			_, err = v.Validate(job)
			if tc.wantError {
				must.ErrorContains(t, err, "group selections require all servers")
			} else {
				must.NoError(t, err)
			}
			if tc.region == "other" {
				job.Multiregion = &structs.Multiregion{Regions: []*structs.MultiregionRegion{{Name: "global"}, {Name: "other"}}}
				must.False(t, v.isEligibleForGroupSelections(job))
			}
		})
	}
}
