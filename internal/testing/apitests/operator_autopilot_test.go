// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestAPI_OperatorAutopilotGetSetConfiguration(t *testing.T) {
	ci.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *api.AutopilotConfiguration
	var err error

	f := func() error {
		config, _, err = operator.AutopilotGetConfiguration(nil)
		return err
	}
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(f)))
	must.True(t, config.CleanupDeadServers)

	// Change a config setting
	newConf := &api.AutopilotConfiguration{CleanupDeadServers: false}
	_, err = operator.AutopilotSetConfiguration(newConf, nil)
	must.NoError(t, err)

	config, _, err = operator.AutopilotGetConfiguration(nil)
	must.NoError(t, err)
	must.False(t, config.CleanupDeadServers)
}

func TestAPI_OperatorAutopilotCASConfiguration(t *testing.T) {
	ci.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *api.AutopilotConfiguration
	var err error
	f := func() error {
		config, _, err = operator.AutopilotGetConfiguration(nil)
		return err
	}
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(f)))
	must.True(t, config.CleanupDeadServers)

	// Pass an invalid ModifyIndex
	{
		newConf := &api.AutopilotConfiguration{
			CleanupDeadServers: false,
			ModifyIndex:        config.ModifyIndex - 1,
		}
		var apState bool
		apState, _, err = operator.AutopilotCASConfiguration(newConf, nil)
		must.NoError(t, err)
		must.False(t, apState)
	}

	// Pass a valid ModifyIndex
	{
		newConf := &api.AutopilotConfiguration{
			CleanupDeadServers: false,
			ModifyIndex:        config.ModifyIndex,
		}
		var apState bool
		apState, _, err = operator.AutopilotCASConfiguration(newConf, nil)
		must.NoError(t, err)
		must.True(t, apState)
	}
}

func TestAPI_OperatorAutopilotServerHealth(t *testing.T) {
	ci.Parallel(t)
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.Server.RaftProtocol = 3
	})
	defer s.Stop()

	operator := c.Operator()
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := operator.AutopilotServerHealth(nil)
		if err != nil {
			return false, err
		}

		if len(out.Servers) != 1 ||
			!out.Servers[0].Healthy ||
			out.Servers[0].Name != fmt.Sprintf("%s.global", s.Config.NodeName) {
			return false, fmt.Errorf("%v", out)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
