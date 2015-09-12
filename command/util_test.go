package command

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
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

func testAgent(t *testing.T) (*agent.Agent, *agent.HTTPServer, *api.Client, string) {
	// Make the agent
	conf := nextConfig()
	conf.DevMode = true
	a, err := agent.NewAgent(conf, os.Stderr)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	http, err := agent.NewHTTPServer(a, conf, os.Stderr)
	if err != nil {
		a.Shutdown()
		t.Fatalf("err: %s", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", conf.Ports.HTTP)

	// Make a client
	clientConf := api.DefaultConfig()
	clientConf.URL = url
	client, err := api.NewClient(clientConf)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	waitForLeader(t, client)
	return a, http, client, url
}

func waitForLeader(t *testing.T, client *api.Client) {
	testutil.WaitForResult(func() (bool, error) {
		if _, err := client.Status().Leader(); err != nil {
			return false, err
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("timeout waiting for leader: %s", err)
	})
}
