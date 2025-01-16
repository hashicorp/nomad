// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package volume_mounts

import (
	"fmt"
	"testing"
	"time"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
)

// TestVolumeMounts exercises host volume and Docker volume functionality for
// the exec and docker task driver, particularly around mounting locations
// within the container and how this is exposed to the user.
func TestVolumeMounts(t *testing.T) {

	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	sub, stop := jobs3.Submit(t, "./input/volumes.nomad",
		jobs3.Verbose(true),
		jobs3.Timeout(30*time.Second), // exec tasks take longer to start
	)

	allocs := sub.Allocs()
	must.Len(t, 1, allocs)
	allocID0 := allocs[0].ID
	nodeID := allocs[0].NodeID

	oldPath := fmt.Sprintf("/tmp/foo/%s", allocID0)
	logs := sub.Exec("group", "docker_task", []string{"cat", oldPath})
	must.StrContains(t, logs.Stdout, allocID0)

	logs = sub.Exec("group", "exec_task", []string{"cat", oldPath})
	must.StrContains(t, logs.Stdout, allocID0)

	stop()

	sub, stop = jobs3.Submit(t, "./input/volumes.nomad",
		jobs3.Verbose(true),
		jobs3.Timeout(30*time.Second), // exec tasks take longer to start
		jobs3.ReplaceInJobSpec("${attr.kernel.name}", "${node.unique.id}"),
		jobs3.ReplaceInJobSpec("linux", nodeID),
	)

	allocs = sub.Allocs()
	must.Len(t, 1, allocs)
	allocID1 := allocs[0].ID

	newPath := fmt.Sprintf("/tmp/foo/%s", allocID1)

	logs = sub.Exec("group", "docker_task", []string{"cat", newPath})
	must.StrContains(t, logs.Stdout, allocID1,
		must.Sprintf("new alloc data is missing from docker_task, got: %s", logs.Stdout))

	logs = sub.Exec("group", "docker_task", []string{"cat", oldPath})
	must.StrContains(t, logs.Stdout, allocID0,
		must.Sprintf("previous alloc data is missing from docker_task, got: %s", logs.Stdout))

	logs = sub.Exec("group", "exec_task", []string{"cat", newPath})
	must.StrContains(t, logs.Stdout, allocID1,
		must.Sprintf("new alloc data is missing from exec_task, got: %s", logs.Stdout))

	logs = sub.Exec("group", "exec_task", []string{"cat", oldPath})
	must.StrContains(t, logs.Stdout, allocID0,
		must.Sprintf("previous alloc data is missing from exec_task, got: %s", logs.Stdout))
}
