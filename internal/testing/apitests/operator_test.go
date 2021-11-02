package apitests

import (
	"testing"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/require"
)

func TestAPI_OperatorSchedulerGetSetConfiguration(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *api.SchedulerConfigurationResponse
	retry.Run(t, func(r *retry.R) {
		var err error
		config, _, err = operator.SchedulerGetConfiguration(nil)
		r.Check(err)
	})
	require.True(config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	require.False(config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	require.False(config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)

	// Change a config setting
	newConf := &api.SchedulerConfiguration{
		PreemptionConfig: api.PreemptionConfig{
			SystemSchedulerEnabled:  false,
			BatchSchedulerEnabled:   true,
			ServiceSchedulerEnabled: true,
		},
	}
	resp, wm, err := operator.SchedulerSetConfiguration(newConf, nil)
	require.Nil(err)
	require.NotZero(wm.LastIndex)
	// non CAS requests should update on success
	require.True(resp.Updated)

	config, _, err = operator.SchedulerGetConfiguration(nil)
	require.Nil(err)
	require.False(config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	require.True(config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	require.True(config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
}

func TestAPI_OperatorSchedulerCASConfiguration(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *api.SchedulerConfigurationResponse
	retry.Run(t, func(r *retry.R) {
		var err error
		config, _, err = operator.SchedulerGetConfiguration(nil)
		r.Check(err)
	})
	require.True(config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	require.False(config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	require.False(config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)

	// Pass an invalid ModifyIndex
	{
		newConf := &api.SchedulerConfiguration{
			PreemptionConfig: api.PreemptionConfig{SystemSchedulerEnabled: false, BatchSchedulerEnabled: true},
			ModifyIndex:      config.SchedulerConfig.ModifyIndex - 1,
		}
		resp, wm, err := operator.SchedulerCASConfiguration(newConf, nil)
		require.Nil(err)
		require.NotZero(wm.LastIndex)
		require.False(resp.Updated)
	}

	// Pass a valid ModifyIndex
	{
		newConf := &api.SchedulerConfiguration{
			PreemptionConfig: api.PreemptionConfig{SystemSchedulerEnabled: false, BatchSchedulerEnabled: true},
			ModifyIndex:      config.SchedulerConfig.ModifyIndex,
		}
		resp, wm, err := operator.SchedulerCASConfiguration(newConf, nil)
		require.Nil(err)
		require.NotZero(wm.LastIndex)
		require.True(resp.Updated)
	}

	config, _, err := operator.SchedulerGetConfiguration(nil)
	require.Nil(err)
	require.False(config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	require.True(config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	require.False(config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
}
