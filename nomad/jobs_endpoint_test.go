package nomad

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestJobs_Statuses_ACL(t *testing.T) {
	s, _, cleanup := TestACLServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	insufficientToken := mock.CreatePolicyAndToken(t, s.State(), 1, "job-lister",
		mock.NamespacePolicy("default", "", []string{"list-jobs"}))
	happyToken := mock.CreatePolicyAndToken(t, s.State(), 2, "job-reader",
		mock.NamespacePolicy("default", "", []string{"read-job"}))

	for _, tc := range []struct {
		name, token, err string
	}{
		{"no token", "", "Permission denied"},
		{"insufficient perms", insufficientToken.SecretID, "Permission denied"},
		{"happy token", happyToken.SecretID, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.JobsStatusesRequest{}
			req.QueryOptions.Region = "global"
			req.QueryOptions.AuthToken = tc.token

			var resp structs.JobsStatusesResponse
			err := s.RPC("Jobs.Statuses", &req, &resp)

			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func TestJobs_Statuses(t *testing.T) {
	s, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	// method under test
	doRequest := func(t *testing.T, req *structs.JobsStatusesRequest) (resp structs.JobsStatusesResponse) {
		t.Helper()
		must.NotNil(t, req, must.Sprint("request must not be nil"))
		req.QueryOptions.Region = "global"
		must.NoError(t, s.RPC("Jobs.Statuses", req, &resp))
		return resp
	}

	// job helpers
	deleteJob := func(t *testing.T, job *structs.Job) {
		t.Helper()
		idx, err := s.State().LatestIndex()
		must.NoError(t, err)
		err = s.State().DeleteJob(idx+1, job.Namespace, job.ID)
		if err != nil && err.Error() == "job not found" {
			return
		}
		must.NoError(t, err)
	}
	upsertJob := func(t *testing.T, job *structs.Job) {
		idx, err := s.State().LatestIndex()
		must.NoError(t, err)
		err = s.State().UpsertJob(structs.MsgTypeTestSetup, idx+1, nil, job)
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

	// set up 5 jobs
	// they should be in order in state, due to lexicographical indexing
	jobs := make([]*structs.Job, 5)
	var deleteJob0, deleteJob1, deleteJob2 func()
	jobs[0], deleteJob0 = createJob(t, "job0")
	jobs[1], deleteJob1 = createJob(t, "job1")
	jobs[2], deleteJob2 = createJob(t, "job2")
	jobs[3], _ = createJob(t, "job3")
	jobs[4], _ = createJob(t, "job4")

	// request all jobs
	resp := doRequest(t, &structs.JobsStatusesRequest{})
	must.Len(t, 5, resp.Jobs)

	// make sure our state order assumption is correct
	for i, j := range resp.Jobs {
		must.Eq(t, jobs[i].ID, j.ID, must.Sprintf("jobs not in order; idx=%d", i))
	}

	// test various single-job requests

	for _, tc := range []struct {
		name   string
		qo     structs.QueryOptions
		jobs   []structs.NamespacedID
		expect *structs.Job
	}{
		{
			name: "page 1",
			qo: structs.QueryOptions{
				PerPage: 1,
			},
			expect: jobs[0],
		},
		{
			name: "page 2",
			qo: structs.QueryOptions{
				PerPage:   1,
				NextToken: "default." + jobs[1].ID,
			},
			expect: jobs[1],
		},
		{
			name: "reverse",
			qo: structs.QueryOptions{
				PerPage: 1,
				Reverse: true,
			},
			expect: jobs[len(jobs)-1],
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp = doRequest(t, &structs.JobsStatusesRequest{
				QueryOptions: tc.qo,
				Jobs:         tc.jobs,
			})
			must.Len(t, 1, resp.Jobs, must.Sprint("expect only one job"))
			must.Eq(t, tc.expect.ID, resp.Jobs[0].ID)
		})
	}

	// test blocking queries

	// this endpoint should only unblock if something relevant changes.
	// "something relevant" is why this seemingly redundant blocking-query
	// testing is done here, as the logic to determine what is "relevant" is
	// specific to this endpoint, meaning the latest ModifyIndex on each
	// job/alloc/deployment seen while iterating, i.e. those "on-page".

	// blocking query helpers
	startQuery := func(t *testing.T, req *structs.JobsStatusesRequest) context.Context {
		t.Helper()
		if req == nil {
			req = &structs.JobsStatusesRequest{}
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
		// unless some other test coincidentally frees it up
		go func() {
			resp = doRequest(t, req)
			cancel()
		}()
		// give it a moment for the rpc to actually start up
		// FLAKE ALERT: if this job is flaky, this might be why.
		time.Sleep(time.Millisecond * 10)
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
		idx, err := s.State().LatestIndex()
		must.NoError(t, err)
		a := mock.MinAllocForJob(job)
		must.NoError(t,
			s.State().UpsertAllocs(structs.AllocUpdateRequestType, idx+1, []*structs.Allocation{a}),
			must.Sprintf("error creating alloc for job %s", job.ID))
		t.Cleanup(func() {
			idx, err = s.State().Index("allocs")
			test.NoError(t, err)
			test.NoError(t, s.State().DeleteEval(idx, []string{}, []string{a.ID}, false))
		})
	}
	createDeployment := func(t *testing.T, job *structs.Job) {
		t.Helper()
		idx, err := s.State().LatestIndex()
		must.NoError(t, err)
		deploy := mock.Deployment()
		deploy.JobID = job.ID
		must.NoError(t, s.State().UpsertDeployment(idx+1, deploy))
		t.Cleanup(func() {
			test.NoError(t, s.State().DeleteDeployment(idx+1, []string{deploy.ID}))
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
			req := &structs.JobsStatusesRequest{}
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
