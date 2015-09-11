package command

import (
	"sync/atomic"
	"testing"

	"github.com/hashicorp/nomad/command/agent"
)

var offset uint64

func nextConfig() *agent.Config {
	idx := int(atomic.AddUint64(&offset, 1))
	conf := agent.DefaultConfig()

	conf.Region = "region1"
	conf.Datacenter = "dc1"
	conf.NodeName = fmt.Sprintf("node%d", idx)
	conf.BindAddr = "127.0.0.1"
	conf.Server.Bootstrap = true
	conf.Server.Enabled = true
	conf.Client.Enabled = false
	conf.Ports.HTTP = 30000 + idx
	conf.Ports.Serf = 32000 + idx
	conf.Ports.RPC = 31000 + idx

	return conf
}

func testAgent(t *testing.T) *agent.Agent {
	conf := nextConfig()
	agent, err := agent.Create(conf)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	return agent
}
