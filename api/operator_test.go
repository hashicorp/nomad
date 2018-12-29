package api

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestAPI_OperatorSchedulerGetSetConfiguration(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *SchedulerConfigurationResponse
	retry.Run(t, func(r *retry.R) {
		var err error
		config, _, err = operator.SchedulerGetConfiguration(nil)
		r.Check(err)
	})
	require.True(config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)

	// Change a config setting
	newConf := &SchedulerConfiguration{PreemptionConfig: PreemptionConfig{SystemSchedulerEnabled: false}}
	resp, wm, err := operator.SchedulerSetConfiguration(newConf, nil)
	require.Nil(err)
	require.NotZero(wm.LastIndex)
	require.False(resp.Updated)

	config, _, err = operator.SchedulerGetConfiguration(nil)
	require.Nil(err)
	require.False(config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
}

func TestAPI_OperatorSchedulerCASConfiguration(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *SchedulerConfigurationResponse
	retry.Run(t, func(r *retry.R) {
		var err error
		config, _, err = operator.SchedulerGetConfiguration(nil)
		r.Check(err)
	})
	require.True(config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)

	// Pass an invalid ModifyIndex
	{
		newConf := &SchedulerConfiguration{
			PreemptionConfig: PreemptionConfig{SystemSchedulerEnabled: false},
			ModifyIndex:      config.SchedulerConfig.ModifyIndex - 1,
		}
		resp, wm, err := operator.SchedulerCASConfiguration(newConf, nil)
		require.Nil(err)
		require.NotZero(wm.LastIndex)
		require.False(resp.Updated)
	}

	// Pass a valid ModifyIndex
	{
		newConf := &SchedulerConfiguration{
			PreemptionConfig: PreemptionConfig{SystemSchedulerEnabled: false},
			ModifyIndex:      config.SchedulerConfig.ModifyIndex,
		}
		resp, wm, err := operator.SchedulerCASConfiguration(newConf, nil)
		require.Nil(err)
		require.NotZero(wm.LastIndex)
		require.True(resp.Updated)
	}
}
