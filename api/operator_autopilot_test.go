package api

import (
	"testing"

	"fmt"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestAPI_OperatorAutopilotGetSetConfiguration(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	var config *AutopilotConfiguration
	retry.Run(t, func(r *retry.R) {
		var err error
		config, err = operator.AutopilotGetConfiguration(nil)
		r.Check(err)
	})
	assert.True(config.CleanupDeadServers)

	// Change a config setting
	newConf := &AutopilotConfiguration{CleanupDeadServers: false}
	err := operator.AutopilotSetConfiguration(newConf, nil)
	assert.Nil(err)

	config, err = operator.AutopilotGetConfiguration(nil)
	assert.Nil(err)
	assert.False(config.CleanupDeadServers)
}

func TestAPI_OperatorAutopilotCASConfiguration(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	config, err := operator.AutopilotGetConfiguration(nil)
	assert.Nil(err)
	assert.True(config.CleanupDeadServers)

	// Pass an invalid ModifyIndex
	{
		newConf := &AutopilotConfiguration{
			CleanupDeadServers: false,
			ModifyIndex:        config.ModifyIndex - 1,
		}
		resp, err := operator.AutopilotCASConfiguration(newConf, nil)
		assert.Nil(err)
		assert.False(resp)
	}

	// Pass a valid ModifyIndex
	{
		newConf := &AutopilotConfiguration{
			CleanupDeadServers: false,
			ModifyIndex:        config.ModifyIndex,
		}
		resp, err := operator.AutopilotCASConfiguration(newConf, nil)
		assert.Nil(err)
		assert.True(resp)
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
		out, err := operator.AutopilotServerHealth(nil)
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
