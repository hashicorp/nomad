// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestAPI_OperatorSchedulerGetSetConfiguration(t *testing.T) {
	ci.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *api.SchedulerConfigurationResponse
	var err error
	f := func() error {
		config, _, err = operator.SchedulerGetConfiguration(nil)
		return err
	}
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(f)))
	must.True(t, config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	must.False(t, config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	must.False(t, config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)

	// Change a config setting
	newConf := &api.SchedulerConfiguration{
		PreemptionConfig: api.PreemptionConfig{
			SystemSchedulerEnabled:  false,
			BatchSchedulerEnabled:   true,
			ServiceSchedulerEnabled: true,
		},
	}
	resp, wm, err := operator.SchedulerSetConfiguration(newConf, nil)
	must.NoError(t, err)
	must.Positive(t, wm.LastIndex)
	must.True(t, resp.Updated) // non CAS requests should update on success

	config, _, err = operator.SchedulerGetConfiguration(nil)
	must.NoError(t, err)
	must.False(t, config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	must.True(t, config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	must.True(t, config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
}

func TestAPI_OperatorSchedulerCASConfiguration(t *testing.T) {
	ci.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *api.SchedulerConfigurationResponse
	var err error
	f := func() error {
		config, _, err = operator.SchedulerGetConfiguration(nil)
		return err
	}
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(f)))
	must.True(t, config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	must.False(t, config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	must.False(t, config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)

	// Pass an invalid ModifyIndex
	{
		newConf := &api.SchedulerConfiguration{
			PreemptionConfig: api.PreemptionConfig{SystemSchedulerEnabled: false, BatchSchedulerEnabled: true},
			ModifyIndex:      config.SchedulerConfig.ModifyIndex - 1,
		}
		var resp *api.SchedulerSetConfigurationResponse
		var wm *api.WriteMeta
		resp, wm, err = operator.SchedulerCASConfiguration(newConf, nil)
		must.NoError(t, err)
		must.Positive(t, wm.LastIndex)
		must.False(t, resp.Updated)
	}

	// Pass a valid ModifyIndex
	{
		newConf := &api.SchedulerConfiguration{
			PreemptionConfig: api.PreemptionConfig{SystemSchedulerEnabled: false, BatchSchedulerEnabled: true},
			ModifyIndex:      config.SchedulerConfig.ModifyIndex,
		}
		var resp *api.SchedulerSetConfigurationResponse
		var wm *api.WriteMeta
		resp, wm, err = operator.SchedulerCASConfiguration(newConf, nil)
		must.NoError(t, err)
		must.Positive(t, wm.LastIndex)
		must.True(t, resp.Updated)
	}

	config, _, err = operator.SchedulerGetConfiguration(nil)
	must.NoError(t, err)
	must.False(t, config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	must.True(t, config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	must.False(t, config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
}
