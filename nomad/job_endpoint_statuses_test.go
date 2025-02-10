// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestJob_Statuses_ACL(t *testing.T) {
	s, _, cleanup := TestACLServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	job1 := mock.MinJob()
	job2 := mock.MinJob()
	job2.Namespace = "infra"
	must.NoError(t, s.State().UpsertNamespaces(100, []*structs.Namespace{{Name: "infra"}}))
	must.NoError(t, s.State().UpsertJob(structs.MsgTypeTestSetup, 101, nil, job1))
	must.NoError(t, s.State().UpsertJob(structs.MsgTypeTestSetup, 102, nil, job2))

	insufficientToken := mock.CreatePolicyAndToken(t, s.State(), 1, "job-lister",
		mock.NamespacePolicy("default", "", []string{"list-jobs"}))
	happyToken := mock.CreatePolicyAndToken(t, s.State(), 2, "job-reader",
		mock.NamespacePolicy("*", "", []string{"read-job"}))

	for _, tc := range []struct {
		name, token, err, ns string
		expectJobs           int
	}{
		{"no token", "", "Permission denied", "", 0},
		{"insufficient perms", insufficientToken.SecretID, "Permission denied", "", 0},
		{"happy token specific ns", happyToken.SecretID, "", "infra", 1},
		{"happy token wildcard ns", happyToken.SecretID, "", "*", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.JobStatusesRequest{}
			req.QueryOptions.Region = "global"
			req.QueryOptions.AuthToken = tc.token
			req.QueryOptions.Namespace = tc.ns

			var resp structs.JobStatusesResponse
			err := s.RPC("Job.Statuses", &req, &resp)

			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
				must.Len(t, tc.expectJobs, resp.Jobs,
					must.Sprint("expected jobs to be filtered by namespace"))
			}
		})
	}
}

func TestJob_Statuses(t *testing.T) {
	s, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	// method under test
	doRequest := func(t *testing.T, req *structs.JobStatusesRequest) (resp structs.JobStatusesResponse) {
		t.Helper()
		must.NotNil(t, req, must.Sprint("request must not be nil"))
		req.QueryOptions.Region = "global"
		must.NoError(t, s.RPC("Job.Statuses", req, &resp))
		return resp
	}

	// increment state index helper
	incIdx := func(t *testing.T) uint64 {
		t.Helper()
		idx, err := s.State().LatestIndex()
		must.NoError(t, err)
		return idx + 1
	}

	// job helpers
	deleteJob := func(t *testing.T, job *structs.Job) {
		t.Helper()
		err := s.State().DeleteJob(incIdx(t), job.Namespace, job.ID)
		if err != nil && err.Error() == "job not found" {
			return
		}
		must.NoError(t, err)
	}
	upsertJob := func(t *testing.T, job *structs.Job) {
		t.Helper()
		err := s.State().UpsertJob(structs.MsgTypeTestSetup, incIdx(t), nil, job)
		must.NoError(t, err)
	}
	createJob := func(t *testing.T, id string) (job *structs.Job, cleanup func()) {
		t.Helper()
		job = mock.MinJob()
		if id != "" {
			job.ID = id
		}
		upsertJob(t, job)
		cleanup = func() {
			deleteJob(t, job)
		}
		t.Cleanup(cleanup)
		return job, cleanup
	}

	// this little cutie sets the latest state index to a predictable value,
	// to ensure the below jobs span the boundary from 999->1000 which would
	// break pagination without proper uint64 NextToken (ModifyIndex) comparison
	must.NoError(t, s.State().UpsertNamespaces(996, nil))

	// set up some jobs
	// they should be in this order in state using the "modify_index" index,
	// but the RPC will return them in reverse order by default.
	jobs := make([]*structs.Job, 5)
	var deleteJob0, deleteJob1, deleteJob2 func()
	jobs[0], deleteJob0 = createJob(t, "job0")
	jobs[1], deleteJob1 = createJob(t, "job1")
	jobs[2], deleteJob2 = createJob(t, "job2")
	jobs[3], _ = createJob(t, "job3")
	jobs[4], _ = createJob(t, "job4")

	// request all jobs
	resp := doRequest(t, &structs.JobStatusesRequest{})
	must.Len(t, 5, resp.Jobs)

	// make sure our state order assumption is correct
	for i, j := range resp.Jobs {
		reverse := len(jobs) - i - 1
		must.Eq(t, jobs[reverse].ID, j.ID, must.Sprintf("jobs not in order; idx=%d", i))
	}

	// test various single-job requests

	for _, tc := range []struct {
		name       string
		qo         structs.QueryOptions
		jobs       []structs.NamespacedID
		expect     *structs.Job
		expectNext uint64 // NextToken (ModifyIndex)
	}{
		{
			name: "page 1",
			qo: structs.QueryOptions{
				PerPage: 1,
			},
			expect:     jobs[4],
			expectNext: jobs[3].ModifyIndex,
		},
		{
			name: "page 2",
			qo: structs.QueryOptions{
				PerPage:   1,
				NextToken: strconv.FormatUint(jobs[3].ModifyIndex, 10),
			},
			expect:     jobs[3],
			expectNext: jobs[2].ModifyIndex,
		},
		{
			name: "reverse",
			qo: structs.QueryOptions{
				PerPage: 1,
				Reverse: true,
			},
			expect:     jobs[0],
			expectNext: jobs[1].ModifyIndex,
		},
		{
			name: "filter",
			qo: structs.QueryOptions{
				Filter: "ID == " + jobs[0].ID,
			},
			expect: jobs[0],
		},
		{
			name: "specific",
			jobs: []structs.NamespacedID{
				jobs[0].NamespacedID(),
			},
			expect: jobs[0],
		},
		{
			name: "missing",
			jobs: []structs.NamespacedID{
				{
					ID:        "do-not-exist",
					Namespace: "anywhere",
				},
			},
			expect: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp = doRequest(t, &structs.JobStatusesRequest{
				QueryOptions: tc.qo,
				Jobs:         tc.jobs,
			})
			if tc.expect == nil {
				must.Len(t, 0, resp.Jobs, must.Sprint("expect no jobs"))
			} else {
				must.Len(t, 1, resp.Jobs, must.Sprint("expect only one job"))
				must.Eq(t, tc.expect.ID, resp.Jobs[0].ID)
			}
			expectToken := ""
			if tc.expectNext > 0 {
				expectToken = strconv.FormatUint(tc.expectNext, 10)
			}
			must.Eq(t, expectToken, resp.NextToken)
		})
	}

	// test blocking queries

	// this endpoint should only unblock if something relevant changes.
	// "something relevant" is why this seemingly redundant blocking-query
	// testing is done here, as the logic to determine what is "relevant" is
	// specific to this endpoint, meaning the latest ModifyIndex on each
	// job/alloc/deployment seen while iterating, i.e. those "on-page".

	// blocking query helpers
	startQuery := func(t *testing.T, req *structs.JobStatusesRequest) context.Context {
		t.Helper()
		if req == nil {
			req = &structs.JobStatusesRequest{}
		}
		// context to signal when the query unblocks
		// mustBlock and mustUnblock below work by checking ctx.Done()
		ctx, cancel := context.WithCancel(context.Background())
		// default latest index to induce blocking
		if req.QueryOptions.MinQueryIndex == 0 {
			idx, err := s.State().LatestIndex()
			must.NoError(t, err)
			req.QueryOptions.MinQueryIndex = idx
		}
		// start the query
		// note: queries that are expected to remain blocked leak this goroutine
		// unless some other test (or cleanup) coincidentally unblocks it
		go func() {
			resp = doRequest(t, req)
			cancel()
		}()
		// give it a moment for the rpc to actually start up and begin blocking
		// FLAKE ALERT: if this job is flaky, this might be why.
		time.Sleep(time.Millisecond * 100)
		return ctx
	}
	mustBlock := func(t *testing.T, ctx context.Context) {
		t.Helper()
		timer := time.NewTimer(time.Millisecond * 200)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			t.Fatal("query should be blocked")
		case <-timer.C:
		}
	}
	mustUnblock := func(t *testing.T, ctx context.Context) {
		t.Helper()
		timer := time.NewTimer(time.Millisecond * 200)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
			t.Fatal("query should have unblocked")
		}
	}

	// alloc and deployment helpers
	createAlloc := func(t *testing.T, job *structs.Job) {
		t.Helper()
		a := mock.MinAllocForJob(job)
		must.NoError(t,
			s.State().UpsertAllocs(structs.AllocUpdateRequestType, incIdx(t), []*structs.Allocation{a}),
			must.Sprintf("error creating alloc for job %s", job.ID))
		t.Cleanup(func() {
			test.NoError(t, s.State().DeleteEval(incIdx(t), []string{}, []string{a.ID}, false))
		})
	}
	createDeployment := func(t *testing.T, job *structs.Job) {
		t.Helper()
		deploy := mock.Deployment()
		deploy.JobID = job.ID
		must.NoError(t, s.State().UpsertDeployment(incIdx(t), deploy))
		t.Cleanup(func() {
			test.NoError(t, s.State().DeleteDeployment(incIdx(t), []string{deploy.ID}))
		})
	}

	// these must be run in order, as they affect outer-scope state.

	for _, tc := range []struct {
		name  string
		watch *structs.Job                      // optional specific job to query
		run   func(*testing.T)                  // run after starting the blocking query
		check func(*testing.T, context.Context) // mustBlock or mustUnblock
	}{
		{
			name:  "get all jobs",
			check: mustBlock,
		},
		{
			name: "delete job",
			run: func(_ *testing.T) {
				deleteJob0()
			},
			check: mustUnblock,
		},
		{
			name: "change job",
			run: func(t *testing.T) {
				jobs[1].Name = "job1-new-name"
				upsertJob(t, jobs[1])
			},
			check: mustUnblock,
		},
		{
			name: "new job",
			run: func(t *testing.T) {
				createJob(t, "new1")
			},
			check: mustUnblock,
		},
		{
			name:  "delete job off page",
			watch: jobs[2],
			run: func(_ *testing.T) {
				deleteJob1()
			},
			check: mustBlock,
		},
		{
			name:  "delete job on page",
			watch: jobs[2],
			run: func(_ *testing.T) {
				deleteJob2()
			},
			check: mustUnblock,
		},
		{
			name:  "new alloc on page",
			watch: jobs[3],
			run: func(t *testing.T) {
				createAlloc(t, jobs[3])
			},
			check: mustUnblock,
		},
		{
			name:  "new alloc off page",
			watch: jobs[3],
			run: func(t *testing.T) {
				createAlloc(t, jobs[4])
			},
			check: mustBlock,
		},
		{
			name:  "new deployment on page",
			watch: jobs[3],
			run: func(t *testing.T) {
				createDeployment(t, jobs[3])
			},
			check: mustUnblock,
		},
		{
			name:  "new deployment off page",
			watch: jobs[3],
			run: func(t *testing.T) {
				createDeployment(t, jobs[4])
			},
			check: mustBlock,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.JobStatusesRequest{}
			if tc.watch != nil {
				req.Jobs = []structs.NamespacedID{tc.watch.NamespacedID()}
			}
			ctx := startQuery(t, req)
			if tc.run != nil {
				tc.run(t)
			}
			tc.check(t, ctx)
		})
	}
}
