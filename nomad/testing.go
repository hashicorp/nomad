package nomad

import (
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	testing "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/lib/freeport"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/pluginutils/catalog"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	nodeNumber uint32 = 0
)

func TestACLServer(t testing.T, cb func(*Config)) (*Server, *structs.ACLToken) {
	server := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
		if cb != nil {
			cb(c)
		}
	})
	token := mock.ACLManagementToken()
	err := server.State().BootstrapACLTokens(1, 0, token)
	if err != nil {
		t.Fatalf("failed to bootstrap ACL token: %v", err)
	}
	return server, token
}

func TestServer(t testing.T, cb func(*Config)) *Server {
	// Setup the default settings
	config := DefaultConfig()
	config.Logger = testlog.HCLogger(t)
	config.Build = "0.8.0+unittest"
	config.DevMode = true
	nodeNum := atomic.AddUint32(&nodeNumber, 1)
	config.NodeName = fmt.Sprintf("nomad-%03d", nodeNum)

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

	// Squelch output when -v isn't specified
	config.LogOutput = testlog.NewWriter(t)

	// Tighten the autopilot timing
	config.AutopilotConfig.ServerStabilizationTime = 100 * time.Millisecond
	config.ServerHealthInterval = 50 * time.Millisecond
	config.AutopilotInterval = 100 * time.Millisecond

	// Set the plugin loaders
	config.PluginLoader = catalog.TestPluginLoader(t)
	config.PluginSingletonLoader = singleton.NewSingletonLoader(config.Logger, config.PluginLoader)

	// Invoke the callback if any
	if cb != nil {
		cb(config)
	}

	// Enable raft as leader if we have bootstrap on
	config.RaftConfig.StartAsLeader = !config.DevDisableBootstrap

	catalog := consul.NewMockCatalog(config.Logger)

	for i := 10; i >= 0; i-- {
		// Get random ports
		ports := freeport.GetT(t, 2)
		config.RPCAddr = &net.TCPAddr{
			IP:   []byte{127, 0, 0, 1},
			Port: ports[0],
		}
		config.SerfConfig.MemberlistConfig.BindPort = ports[1]

		// Create server
		server, err := NewServer(config, catalog)
		if err == nil {
			return server
		} else if i == 0 {
			t.Fatalf("err: %v", err)
		} else {
			if server != nil {
				server.Shutdown()
			}
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
		}
	}

	return nil
}

func TestJoin(t testing.T, s1 *Server, other ...*Server) {
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfConfig.MemberlistConfig.BindPort)
	for _, s2 := range other {
		if num, err := s2.Join([]string{addr}); err != nil {
			t.Fatalf("err: %v", err)
		} else if num != 1 {
			t.Fatalf("bad: %d", num)
		}
	}
}
