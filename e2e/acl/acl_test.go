// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
)

func TestACL(t *testing.T) {

	// Wait until we have a usable cluster before running the tests. While the
	// test does not run client workload, some do perform listings of nodes. It
	// is therefore better to wait until we have a node, so these tests can
	// check for a non-empty node list response object.
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	// Run our test cases.
	t.Run("TestACL_Role", testACLRole)
	t.Run("TestACL_TokenExpiration", testACLTokenExpiration)
	t.Run("TestACL_TokenRolePolicyAssignment", testACLTokenRolePolicyAssignment)
}
