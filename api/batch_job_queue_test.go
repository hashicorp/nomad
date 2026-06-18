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
	queue := c.BatchJobQueue()

	// The passthrough queue just returns the unset type
	resp, _, err := queue.Jobs(nil)
	must.NoError(t, err)
	must.Eq(t, resp.Type, "unset")
}

func TestJobs_BatchQueue_Tenants(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	queue := c.BatchJobQueue()

	// The passthrough queue just returns the unset type
	resp, _, err := queue.Tenants(nil)
	must.NoError(t, err)
	must.Eq(t, resp.Type, "unset")
}
