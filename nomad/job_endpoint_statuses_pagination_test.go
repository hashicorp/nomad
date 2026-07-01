// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TestJob_Statuses_Pagination_SharedModifyIndex is a regression test for the
// /v1/jobs/statuses paginator. Its NextToken is built from ModifyIndex alone,
// with no tiebreaker. When several jobs share a ModifyIndex (e.g. written in a
// single Raft transaction) and that group does not fit in one page, the cursor
// cannot advance past the group: the same page repeats (duplicate jobs) and the
// walk never terminates.
//
// Paginating this endpoint must always (a) terminate and (b) return every job
// exactly once, regardless of jobs sharing a ModifyIndex.
func TestJob_Statuses_Pagination_SharedModifyIndex(t *testing.T) {
	ci.Parallel(t)

	s, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	// Write several jobs at a single shared ModifyIndex, as happens when one
	// Raft transaction writes multiple jobs at once.
	const n = 5
	const sharedIndex = 1000
	want := make(map[string]bool, n)
	for i := range n {
		job := mock.MinJob()
		job.ID = fmt.Sprintf("shared-%02d", i)
		must.NoError(t, s.State().UpsertJob(structs.MsgTypeTestSetup, sharedIndex, nil, job))
		want[job.ID] = true
	}

	// Page through the endpoint following NextToken, the way the web UI does.
	// per_page is smaller than the shared group, so the group spans a page
	// boundary -- exactly the case the ModifyIndex-only token cannot handle.
	seen := make(map[string]int, n)
	token := ""
	pages := 0
	// A terminating walk needs ceil(n/perPage) pages; a stalled cursor never
	// stops, so anything well past that means the walk did not terminate.
	maxPages := n * 3
	for {
		pages++
		if pages > maxPages {
			t.Fatalf("pagination did not terminate after %d pages: cursor stuck at token %q (seen=%v)",
				pages, token, seen)
		}

		req := &structs.JobStatusesRequest{}
		req.QueryOptions.Region = "global"
		req.QueryOptions.Namespace = structs.DefaultNamespace
		req.QueryOptions.PerPage = 2
		req.QueryOptions.NextToken = token

		var resp structs.JobStatusesResponse
		must.NoError(t, s.RPC("Job.Statuses", req, &resp))

		for _, j := range resp.Jobs {
			seen[j.ID]++
		}
		if resp.NextToken == "" {
			break
		}
		token = resp.NextToken
	}

	for id := range want {
		must.Eq(t, 1, seen[id],
			must.Sprintf("job %s returned %d time(s) across pages, want exactly 1", id, seen[id]))
	}
	must.Eq(t, n, len(seen), must.Sprintf("walked jobs = %v", seen))
}
