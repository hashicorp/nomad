// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
)

func TestConsul(t *testing.T) {
	// todo: migrate the remaining consul tests

	nomad := e2eutil.NomadClient(t)
	consul := e2eutil.ConsulClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	// setup consul ACL's for WI auth
	e2eutil.SetupConsulACLsForServices(t, consul, consulPolicyServiceInput)
	e2eutil.SetupConsulServiceIntentions(t, consul)
	e2eutil.SetupConsulACLsForTasks(t, consul, "nomad-default", consulPolicyTaskInput)
	e2eutil.SetupConsulJWTAuth(t, consul, nomad.Address(), nil)

	t.Run("testServiceReversion", testServiceReversion)
	t.Run("testAllocRestart", testAllocRestart)
}
