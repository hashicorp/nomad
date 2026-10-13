// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestJobs_BatchQueue_Jobs(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	queue := c.BatchQueue()

	resp, _, err := queue.Jobs(nil)
	must.NoError(t, err)
	must.Eq(t, "passthrough", resp.Type)
}

func TestJobs_BatchQueue_Status(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	queue := c.BatchQueue()

	resp, _, err := queue.Status(nil)
	must.NoError(t, err)
	must.Eq(t, "passthrough", resp.Type)
}
