// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package nomad

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestConsulACLsAPI_hasSufficientPolicy_oss(t *testing.T) {
	ci.Parallel(t)

	try := func(t *testing.T, namespace, task string, token *api.ACLToken, exp bool) {
		logger := testlog.HCLogger(t)
		cAPI := &consulACLsAPI{
			aclClient: consul.NewMockACLsAPI(logger),
			logger:    logger,
		}
		result, err := cAPI.canWriteService(namespace, task, token)
		must.NoError(t, err)
		must.Eq(t, exp, result)
	}

	// In Nomad OSS, group consul namespace will always be empty string.

	t.Run("no namespace with default token", func(t *testing.T) {
		t.Run("no useful policy or role", func(t *testing.T) {
			try(t, "", "service1", consul.ExampleOperatorToken0, false)
		})

		t.Run("working policy only", func(t *testing.T) {
			try(t, "", "service1", consul.ExampleOperatorToken1, true)
		})

		t.Run("working role only", func(t *testing.T) {
			try(t, "", "service1", consul.ExampleOperatorToken4, true)
		})

		t.Run("working service identity only", func(t *testing.T) {
			try(t, "", "service1", consul.ExampleOperatorToken6, true)
		})
	})
}
