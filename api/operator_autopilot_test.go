package api

import (
	"testing"

	"fmt"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestAPI_OperatorAutopilotGetSetConfiguration(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *AutopilotConfiguration
	retry.Run(t, func(r *retry.R) {
		var err error
		config, _, err = operator.AutopilotGetConfiguration(nil)
		r.Check(err)
	})
	require.True(config.CleanupDeadServers)

	// Change a config setting
	newConf := &AutopilotConfiguration{CleanupDeadServers: false}
	_, err := operator.AutopilotSetConfiguration(newConf, nil)
	require.Nil(err)

	config, _, err = operator.AutopilotGetConfiguration(nil)
	require.Nil(err)
	require.False(config.CleanupDeadServers)
}

func TestAPI_OperatorAutopilotCASConfiguration(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *AutopilotConfiguration
	retry.Run(t, func(r *retry.R) {
		var err error
		config, _, err = operator.AutopilotGetConfiguration(nil)
		r.Check(err)
	})
	require.True(config.CleanupDeadServers)

	// Pass an invalid ModifyIndex
	{
		newConf := &AutopilotConfiguration{
			CleanupDeadServers: false,
			ModifyIndex:        config.ModifyIndex - 1,
		}
		resp, _, err := operator.AutopilotCASConfiguration(newConf, nil)
		require.Nil(err)
		require.False(resp)
	}

	// Pass a valid ModifyIndex
	{
		newConf := &AutopilotConfiguration{
			CleanupDeadServers: false,
			ModifyIndex:        config.ModifyIndex,
		}
		resp, _, err := operator.AutopilotCASConfiguration(newConf, nil)
		require.Nil(err)
		require.True(resp)
	}
}

func TestAPI_OperatorAutopilotServerHealth(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.AdvertiseAddrs.RPC = "127.0.0.1"
		c.Server.RaftProtocol = 3
	})
	defer s.Stop()

	operator := c.Operator()
	retry.Run(t, func(r *retry.R) {
		out, _, err := operator.AutopilotServerHealth(nil)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		if len(out.Servers) != 1 ||
			!out.Servers[0].Healthy ||
			out.Servers[0].Name != fmt.Sprintf("%s.global", s.Config.NodeName) {
			r.Fatalf("bad: %v", out)
		}
	})
}
