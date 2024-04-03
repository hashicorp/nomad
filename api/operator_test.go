// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	out, err := operator.RaftGetConfiguration(nil)
	must.NoError(t, err)
	must.Len(t, 1, out.Servers)
	must.True(t, out.Servers[0].Leader)
	must.True(t, out.Servers[0].Voter)
}

func TestOperator_RaftRemovePeerByAddress(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// If we get this error, it proves we sent the address all the way
	// through.
	operator := c.Operator()
	err := operator.RaftRemovePeerByAddress("nope", nil)
	must.ErrorContains(t, err, `address "nope" was not found in the Raft configuration`)
}

func TestOperator_RaftRemovePeerByID(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// If we get this error, it proves we sent the address all the way through.
	operator := c.Operator()
	err := operator.RaftRemovePeerByID("nope", nil)
	must.ErrorContains(t, err, `id "nope" was not found in the Raft configuration`)
}

func TestOperator_SchedulerGetConfiguration(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	schedulerConfig, _, err := c.Operator().SchedulerGetConfiguration(nil)
	must.NoError(t, err)
	must.NotNil(t, schedulerConfig)
}

func TestOperator_SchedulerSetConfiguration(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	newSchedulerConfig := SchedulerConfiguration{
		SchedulerAlgorithm: SchedulerAlgorithmSpread,
		PreemptionConfig: PreemptionConfig{
			SystemSchedulerEnabled:   true,
			SysBatchSchedulerEnabled: true,
			BatchSchedulerEnabled:    true,
			ServiceSchedulerEnabled:  true,
		},
		MemoryOversubscriptionEnabled: true,
		RejectJobRegistration:         true,
		PauseEvalBroker:               true,
	}

	schedulerConfigUpdateResp, _, err := c.Operator().SchedulerSetConfiguration(&newSchedulerConfig, nil)
	must.NoError(t, err)
	must.True(t, schedulerConfigUpdateResp.Updated)

	// We can't exactly predict the query meta responses, so we test fields individually.
	schedulerConfig, _, err := c.Operator().SchedulerGetConfiguration(nil)
	must.NoError(t, err)
	must.Eq(t, SchedulerAlgorithmSpread, schedulerConfig.SchedulerConfig.SchedulerAlgorithm)
	must.True(t, schedulerConfig.SchedulerConfig.PauseEvalBroker)
	must.True(t, schedulerConfig.SchedulerConfig.RejectJobRegistration)
	must.True(t, schedulerConfig.SchedulerConfig.MemoryOversubscriptionEnabled)
	must.Eq(t, schedulerConfig.SchedulerConfig.PreemptionConfig, newSchedulerConfig.PreemptionConfig)
}

func TestOperator_AutopilotState(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()

	// Make authenticated request.
	_, _, err := operator.AutopilotServerHealth(nil)
	must.NoError(t, err)

	// Make unauthenticated request.
	c.SetSecretID("")
	_, _, err = operator.AutopilotServerHealth(nil)
	must.ErrorContains(t, err, "403")
}
