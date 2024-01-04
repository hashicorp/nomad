// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

type ConsulWITest struct {
	nomadAddress string
}

// testConsulWI_Service_and_Task asserts we can
// - configure Consul correctly with setup consul -y command
// - run a job with a service and a task
// - make sure the expected service is registered in Consul
// - get Consul token written to a task secret directory
func (tc *ConsulWITest) testConsulWIServiceAndTask(t *testing.T) {
	const jobFile = "./input/consul_wi_example.nomad"
	jobID := "consul-wi-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Run the setup helper that should configure Consul ACL with default
	// policies, roles, auth method and binding rules.
	jwksURL := fmt.Sprintf("%s/.well-known/jwks.json", tc.nomadAddress)
	_, err := e2eutil.Command("nomad", "setup", "consul", "-y", "-jwks-url", jwksURL)
	must.NoError(t, err)

	// register a job
	err = e2eutil.Register(jobID, jobFile)
	must.NoError(t, err)

	// wait for job to be running
	err = e2eutil.WaitForAllocStatusExpected(jobID, "", []string{structs.AllocClientStatusRunning})
	must.NoError(t, err)

	// get our consul client
	consulClient := e2eutil.ConsulClient(t)

	// check if the service is registered with Consul
	_, _, consulErr := consulClient.Catalog().Service("consul-example", "", nil)
	must.NoError(t, consulErr)

	allocID := e2eutil.SingleAllocID(t, jobID, "", 0)

	// secret dir should contain a Consul token file
	_, err = e2eutil.AllocExec(allocID, "example", "ls example/secrets/consul_token", "default", nil)
	must.NoError(t, err)

	// rendered template should contain a Consul token string
	err = e2eutil.WaitForAllocFile(allocID, "example/local/config.txt", func(content string) bool {
		return len(content) > 12 // CONSUL_TOKEN=actual_consul_token and not a blank string
	}, nil)
	must.NoError(t, err)
}
