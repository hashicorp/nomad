// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestJobs_BatchQueue_Status(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.BatchQueueStatus(nil, nil)
	must.NoError(t, err)
	must.Eq(t, resp.Type, "unset")
}
