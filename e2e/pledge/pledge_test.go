// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pledge

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

func TestPledge(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
		cluster3.Timeout(10*time.Second),
	)

	t.Run("testSleep", testSleep)
	t.Run("testBridgeNetwork", testBridgeNetwork)
	t.Run("testUnveil", testUnveil)
}

func testSleep(t *testing.T) {
	_, cleanup := jobs3.Submit(t, "./input/sleep.hcl")
	t.Cleanup(cleanup)
}

func testBridgeNetwork(t *testing.T) {
	_, cleanup := jobs3.Submit(t, "./input/bridge.hcl")
	t.Cleanup(cleanup)

	ip, port := findService(t, "pybridge")
	address := fmt.Sprintf("http://%s:%d", ip, port)

	curlJob, curlCleanup := jobs3.Submit(t, "./input/curl.hcl",
		jobs3.Var("address", address),
		jobs3.WaitComplete("curl"),
	)
	t.Cleanup(curlCleanup)

	logs := curlJob.TaskLogs("group", "curl")
	must.StrContains(t, logs.Stdout, "<title>bridge mode</title>")
}

func testUnveil(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/unveil.hcl")
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "cat")
	must.StrContains(t, logs.Stdout, "root:x:0:0")
}

// findService returns the service address and port
func findService(t *testing.T, name string) (string, int) {
	services, _, err := e2eutil.NomadClient(t).Services().Get(name, nil)
	must.NoError(t, err)
	return services[0].Address, services[0].Port
}
