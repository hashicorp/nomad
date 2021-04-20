package nomad

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync/atomic"
	"time"

	testing "github.com/mitchellh/go-testing-interface"
	"github.com/pkg/errors"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/freeport"
	"github.com/hashicorp/nomad/helper/pluginutils/catalog"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/version"
)

var (
	nodeNumber uint32 = 0
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
	// Setup the default settings
	config := DefaultConfig()
	config.Logger = testlog.HCLogger(t)
	config.Build = version.Version + "+unittest"
	config.DevMode = true
	config.EnableEventBroker = true
	config.BootstrapExpect = 1
	nodeNum := atomic.AddUint32(&nodeNumber, 1)
	config.NodeName = fmt.Sprintf("nomad-%03d", nodeNum)

	// configer logger
	level := hclog.Trace
	if envLogLevel := os.Getenv("NOMAD_TEST_LOG_LEVEL"); envLogLevel != "" {
		level = hclog.LevelFromString(envLogLevel)
	}
	opts := &hclog.LoggerOptions{
		Level:           level,
		Output:          testlog.NewPrefixWriter(t, config.NodeName+" "),
		IncludeLocation: true,
	}
	config.Logger = hclog.NewInterceptLogger(opts)
	config.LogOutput = opts.Output

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

	// Set the plugin loaders
	config.PluginLoader = catalog.TestPluginLoader(t)
	config.PluginSingletonLoader = singleton.NewSingletonLoader(config.Logger, config.PluginLoader)

	// Disable consul autojoining: tests typically join servers directly
	config.ConsulConfig.ServerAutoJoin = &f

	// Enable fuzzy search API
	config.SearchConfig = &structs.SearchConfig{
		FuzzyEnabled:  true,
		LimitQuery:    20,
		LimitResults:  100,
		MinTermLength: 2,
	}

	// Invoke the callback if any
	if cb != nil {
		cb(config)
	}

	cCatalog := consul.NewMockCatalog(config.Logger)
	cConfigs := consul.NewMockConfigsAPI(config.Logger)
	cACLs := consul.NewMockACLsAPI(config.Logger)

	for i := 10; i >= 0; i-- {
		// Get random ports, need to cleanup later
		ports := freeport.MustTake(2)

		config.RPCAddr = &net.TCPAddr{
			IP:   []byte{127, 0, 0, 1},
			Port: ports[0],
		}
		config.SerfConfig.MemberlistConfig.BindPort = ports[1]

		// Create server
		server, err := NewServer(config, cCatalog, cConfigs, cACLs)
		if err == nil {
			return server, func() {
				ch := make(chan error)
				go func() {
					defer close(ch)

					// Shutdown server
					err := server.Shutdown()
					if err != nil {
						ch <- errors.Wrap(err, "failed to shutdown server")
					}

					freeport.Return(ports)
				}()

				select {
				case e := <-ch:
					if e != nil {
						t.Fatal(e.Error())
					}
				case <-time.After(1 * time.Minute):
					t.Fatal("timed out while shutting down server")
				}
			}
		} else if i == 0 {
			freeport.Return(ports)
			t.Fatalf("err: %v", err)
		} else {
			if server != nil {
				_ = server.Shutdown()
				freeport.Return(ports)
			}
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
		}
	}

	return nil, nil
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
