// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/version"
	testing "github.com/mitchellh/go-testing-interface"
	"github.com/shoenig/test/must"
)

var (
	nodeNumber int32 = 0
)

func TestACLServer(t testing.T, cb func(*Config)) (*Server, *structs.ACLToken, func()) {
	server, cleanup := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
		if cb != nil {
			cb(c)
		}
	})
	token := mock.ACLManagementToken()
	err := server.State().BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, token)
	if err != nil {
		t.Fatalf("failed to bootstrap ACL token: %v", err)
	}
	return server, token, cleanup
}

func TestServer(t testing.T, cb func(*Config)) (*Server, func()) {
	s, c, err := TestServerErr(t, cb)
	must.NoError(t, err, must.Sprint("failed to start test server"))
	return s, c
}

// TestConfigForServer provides a fully functional Config to pass to NewServer()
// It can be changed beforehand to induce different behavior such as specific errors.
func TestConfigForServer(t testing.T) *Config {
	t.Helper()

	// Setup the default settings
	config := DefaultConfig()

	// Setup default enterprise-specific settings, including license
	defaultEnterpriseTestConfig(config)

	config.Build = version.Version + "+unittest"
	config.DevMode = true
	config.DataDir = t.TempDir()
	config.EnableEventBroker = true
	config.BootstrapExpect = 1
	nodeNum := atomic.AddInt32(&nodeNumber, 1)
	config.NodeName = fmt.Sprintf("nomad-%03d", nodeNum)

	// configure logger
	config.Logger, config.LogOutput = testlog.HCLoggerNode(t, nodeNum)

	// Tighten the Serf timing
	config.SerfConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfConfig.MemberlistConfig.RetransmitMult = 2
	config.SerfConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	// Tighten the Raft timing
	config.RaftConfig.LeaderLeaseTimeout = 50 * time.Millisecond
	config.RaftConfig.HeartbeatTimeout = 50 * time.Millisecond
	config.RaftConfig.ElectionTimeout = 50 * time.Millisecond
	config.RaftTimeout = 500 * time.Millisecond

	// Disable Vault
	f := false
	config.VaultConfig.Enabled = &f

	// Tighten the autopilot timing
	config.AutopilotConfig.ServerStabilizationTime = 100 * time.Millisecond
	config.ServerHealthInterval = 50 * time.Millisecond
	config.AutopilotInterval = 100 * time.Millisecond

	// Disable consul autojoining: tests typically join servers directly
	config.ConsulConfig.ServerAutoJoin = &f

	// Enable fuzzy search API
	config.SearchConfig = &structs.SearchConfig{
		FuzzyEnabled:  true,
		LimitQuery:    20,
		LimitResults:  100,
		MinTermLength: 2,
	}

	// Get random ports for RPC and Serf
	ports := ci.PortAllocator.Grab(2)
	config.RPCAddr = &net.TCPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: ports[0],
	}
	config.SerfConfig.MemberlistConfig.BindPort = ports[1]

	// max job submission source size
	config.JobMaxSourceSize = 1e6

	// Default to having concurrent schedulers
	config.NumSchedulers = 2

	return config
}

func TestServerErr(t testing.T, cb func(*Config)) (*Server, func(), error) {
	config := TestConfigForServer(t)
	// Invoke the callback if any
	if cb != nil {
		cb(config)
	}

	cCatalog := consul.NewMockCatalog(config.Logger)
	cConfigs := consul.NewMockConfigsAPI(config.Logger)
	cACLs := consul.NewMockACLsAPI(config.Logger)

	var server *Server
	var err error

	for i := 10; i >= 0; i-- {
		// Create server
		server, err = NewServer(config, cCatalog, cConfigs, cACLs)
		if err == nil {
			return server, func() {
				ch := make(chan error)
				go func() {
					defer close(ch)

					// Shutdown server
					err = server.Shutdown()
					if err != nil {
						ch <- fmt.Errorf("failed to shutdown server: %w", err)
					}
				}()

				select {
				case e := <-ch:
					if e != nil {
						t.Fatal(e.Error())
					}
				case <-time.After(1 * time.Minute):
					t.Fatal("timed out while shutting down server")
				}
			}, nil
		} else if i > 0 {
			if server != nil {
				_ = server.Shutdown()
			}
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
		}

		// if it failed for port reasons, try new ones
		ports := ci.PortAllocator.Grab(2)
		config.RPCAddr = &net.TCPAddr{
			IP:   []byte{127, 0, 0, 1},
			Port: ports[0],
		}
		config.SerfConfig.MemberlistConfig.BindPort = ports[1]
	}

	return nil, nil, fmt.Errorf("error starting test server: %w", err)
}

func TestJoin(t testing.T, servers ...*Server) {
	for i := 0; i < len(servers)-1; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d",
			servers[i].config.SerfConfig.MemberlistConfig.BindPort)

		for j := i + 1; j < len(servers); j++ {
			num, err := servers[j].Join([]string{addr})
			must.NoError(t, err)
			must.Eq(t, 1, num)
		}
	}
}
