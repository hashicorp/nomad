package api

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	out, err := operator.RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.Servers) != 1 ||
		!out.Servers[0].Leader ||
		!out.Servers[0].Voter {
		t.Fatalf("bad: %v", out)
	}
}

func TestOperator_RaftRemovePeerByAddress(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// If we get this error, it proves we sent the address all the way
	// through.
	operator := c.Operator()
	err := operator.RaftRemovePeerByAddress("nope", nil)
	if err == nil || !strings.Contains(err.Error(),
		"address \"nope\" was not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}
}

func TestOperator_RaftRemovePeerByID(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// If we get this error, it proves we sent the address all the way
	// through.
	operator := c.Operator()
	err := operator.RaftRemovePeerByID("nope", nil)
	if err == nil || !strings.Contains(err.Error(),
		"id \"nope\" was not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}
}

func TestOperator_SchedulerGetConfiguration(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	schedulerConfig, _, err := c.Operator().SchedulerGetConfiguration(nil)
	require.Nil(t, err)
	require.NotEmpty(t, schedulerConfig)
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
	require.Nil(t, err)
	require.True(t, schedulerConfigUpdateResp.Updated)

	// We can't exactly predict the query meta responses, so we test fields
	// individually.
	schedulerConfig, _, err := c.Operator().SchedulerGetConfiguration(nil)
	require.Nil(t, err)
	require.Equal(t, schedulerConfig.SchedulerConfig.SchedulerAlgorithm, SchedulerAlgorithmSpread)
	require.True(t, schedulerConfig.SchedulerConfig.PauseEvalBroker)
	require.True(t, schedulerConfig.SchedulerConfig.RejectJobRegistration)
	require.True(t, schedulerConfig.SchedulerConfig.MemoryOversubscriptionEnabled)
	require.Equal(t, newSchedulerConfig.PreemptionConfig, schedulerConfig.SchedulerConfig.PreemptionConfig)
}
