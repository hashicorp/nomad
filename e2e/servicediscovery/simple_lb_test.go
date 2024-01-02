// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package servicediscovery

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func testSimpleLoadBalancing(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	// Generate our unique job ID which will be used for this test.
	jobID := "nsd-simple-lb-replicas-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Register the replicas job.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobSimpleLBReplicas, jobID, "")
	must.Len(t, 3, allocStubs)

	for _, stub := range allocStubs {
		var tag string
		switch stub.TaskGroup {
		case "db_replica_1":
			tag = "r1"
		case "db_replica_2":
			tag = "r2"
		case "db_replica_3":
			tag = "r3"
		}
		expectService := api.ServiceRegistration{
			ServiceName: "db",
			Namespace:   api.DefaultNamespace,
			Datacenter:  "dc1",
			JobID:       jobID,
			AllocID:     stub.ID,
			Tags:        []string{tag},
		}
		filter := fmt.Sprintf("Tags contains %q", tag)
		requireEventuallyNomadService(t, &expectService, filter)
	}

	jobID2 := "nsd-simple-lb-clients" + uuid.Short()
	jobIDs = append(jobIDs, jobID2)

	// Register the clients job.
	allocStubs = e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobSimpleLBClients, jobID2, "")
	must.Len(t, 2, allocStubs)

	for _, stub := range allocStubs {
		var expCount int
		switch stub.TaskGroup {
		case "client_1":
			expCount = 1
		case "client_2":
			expCount = 2
		}
		must.NoError(t, e2eutil.WaitForAllocFile(stub.ID, "cat/output.txt", func(content string) bool {
			count := strings.Count(content, "server ")
			return count == expCount
		}, nil))
	}
}
