// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/queues"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// Not much to test here at the moment
func TestBatchJobQueue_Status(t *testing.T) {
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	// swap the server's queue for a mock
	s.batchJobQueue = &queues.PassthroughQueue{}

	req := structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{
		Region: "global",
	}}
	reply := structs.QueueStatusResponse{}

	err := s.RPC("BatchJobQueue.Status", &req, &reply)
	must.NoError(t, err)
	must.Eq(t, reply.Type, "unset")
	must.Nil(t, reply.Workloads)
}
