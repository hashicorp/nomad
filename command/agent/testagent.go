package agent

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
)

func init() {
	rand.Seed(time.Now().UnixNano()) // seed random number generator
}

// TempDir defines the base dir for temporary directories.
var TempDir = os.TempDir()

// TestAgent encapsulates an Agent with a default configuration and
// startup procedure suitable for testing. It panics if there are errors
// during creation or startup instead of returning errors. It manages a
// temporary data directory which is removed after shutdown.
type TestAgent struct {
	// Name is an optional name of the agent.
	Name string

	// ConfigCallback is an optional callback that allows modification of the
	// configuration before the agent is started.
	ConfigCallback func(*Config)

	// Config is the agent configuration. If Config is nil then
	// TestConfig() is used. If Config.DataDir is set then it is
	// the callers responsibility to clean up the data directory.
	// Otherwise, a temporary data directory is created and removed
	// when Shutdown() is called.
	Config *Config

	// LogOutput is the sink for the logs. If nil, logs are written
	// to os.Stderr.
	LogOutput io.Writer

	// DataDir is the data directory which is used when Config.DataDir
	// is not set. It is created automatically and removed when
	// Shutdown() is called.
	DataDir string

	// Key is the optional encryption key for the keyring.
	Key string

	// Server is a reference to the started HTTP endpoint.
	// It is valid after Start().
	Server *HTTPServer

	// Agent is the embedded Nomad agent.
	// It is valid after Start().
	*Agent

	// Token is auto-bootstrapped if ACLs are enabled
	Token *structs.ACLToken
}

// NewTestAgent returns a started agent with the given name and
// configuration. It panics if the agent could not be started. The
// caller should call Shutdown() to stop the agent and remove temporary
// directories.
func NewTestAgent(name string, configCallback func(*Config)) *TestAgent {
	a := &TestAgent{Name: name, ConfigCallback: configCallback}
	a.Start()
	return a
}

// Start starts a test agent. It panics if the agent could not be started.
func (a *TestAgent) Start() *TestAgent {
	if a.Agent != nil {
		panic("TestAgent already started")
	}
	if a.Config == nil {
		a.Config = a.config()
	}
	if a.Config.DataDir == "" {
		name := "agent"
		if a.Name != "" {
			name = a.Name + "-agent"
		}
		name = strings.Replace(name, "/", "_", -1)
		d, err := ioutil.TempDir(TempDir, name)
		if err != nil {
			panic(fmt.Sprintf("Error creating data dir %s: %s", filepath.Join(TempDir, name), err))
		}
		a.DataDir = d
		a.Config.DataDir = d
		a.Config.NomadConfig.DataDir = d
	}

	for i := 10; i >= 0; i-- {
		pickRandomPorts(a.Config)
		if a.Config.NodeName == "" {
			a.Config.NodeName = fmt.Sprintf("Node %d", a.Config.Ports.RPC)
		}

		// write the keyring
		if a.Key != "" {
			writeKey := func(key, filename string) {
				path := filepath.Join(a.Config.DataDir, filename)
				if err := initKeyring(path, key); err != nil {
					panic(fmt.Sprintf("Error creating keyring %s: %s", path, err))
				}
			}
			writeKey(a.Key, serfKeyring)
		}

		// we need the err var in the next exit condition
		if agent, err := a.start(); err == nil {
			a.Agent = agent
			break
		} else if i == 0 {
			fmt.Println(a.Name, "Error starting agent:", err)
			runtime.Goexit()
		} else {
			if agent != nil {
				agent.Shutdown()
			}
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			fmt.Println(a.Name, "retrying in", wait)
			time.Sleep(wait)
		}

		// Clean out the data dir if we are responsible for it before we
		// try again, since the old ports may have gotten written to
		// the data dir, such as in the Raft configuration.
		if a.DataDir != "" {
			if err := os.RemoveAll(a.DataDir); err != nil {
				fmt.Println(a.Name, "Error resetting data dir:", err)
				runtime.Goexit()
			}
		}
	}

	if a.Config.NomadConfig.Bootstrap && a.Config.Server.Enabled {
		testutil.WaitForResult(func() (bool, error) {
			args := &structs.GenericRequest{}
			var leader string
			err := a.RPC("Status.Leader", args, &leader)
			return leader != "", err
		}, func(err error) {
			panic(fmt.Sprintf("failed to find leader: %v", err))
		})
	} else {
		testutil.WaitForResult(func() (bool, error) {
			req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			_, err := a.Server.AgentSelfRequest(resp, req)
			return err == nil && resp.Code == 200, err
		}, func(err error) {
			panic(fmt.Sprintf("failed OK response: %v", err))
		})
	}

	// Check if ACLs enabled. Use special value of PolicyTTL 0s
	// to do a bypass of this step. This is so we can test bootstrap
	// without having to pass down a special flag.
	if a.Config.ACL.Enabled && a.Config.Server.Enabled && a.Config.ACL.PolicyTTL != 0 {
		a.Token = mock.ACLManagementToken()
		state := a.Agent.server.State()
		if err := state.BootstrapACLTokens(1, 0, a.Token); err != nil {
			panic(fmt.Sprintf("token bootstrap failed: %v", err))
		}
	}
	return a
}

func (a *TestAgent) start() (*Agent, error) {
	if a.LogOutput == nil {
		a.LogOutput = os.Stderr
	}

	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.NewGlobal(metrics.DefaultConfig("service-name"), inm)

	if inm == nil {
		return nil, fmt.Errorf("unable to set up in memory metrics needed for agent initialization")
	}

	agent, err := NewAgent(a.Config, a.LogOutput, inm)
	if err != nil {
		return nil, err
	}

	// Setup the HTTP server
	http, err := NewHTTPServer(agent, a.Config)
	if err != nil {
		return agent, err
	}

	a.Server = http
	return agent, nil
}

// Shutdown stops the agent and removes the data directory if it is
// managed by the test agent.
func (a *TestAgent) Shutdown() error {
	defer func() {
		if a.DataDir != "" {
			os.RemoveAll(a.DataDir)
		}
	}()

	// shutdown agent before endpoints
	a.Server.Shutdown()
	return a.Agent.Shutdown()
}

func (a *TestAgent) HTTPAddr() string {
	if a.Server == nil {
		return ""
	}
	return "http://" + a.Server.Addr
}

func (a *TestAgent) Client() *api.Client {
	conf := api.DefaultConfig()
	conf.Address = a.HTTPAddr()
	c, err := api.NewClient(conf)
	if err != nil {
		panic(fmt.Sprintf("Error creating Nomad API client: %s", err))
	}
	return c
}

// FivePorts returns the first port number of a block of
// five random ports.
func FivePorts() int {
	return 1030 + int(rand.Int31n(6440))*5
}

// pickRandomPorts selects random ports from fixed size random blocks of
// ports. This does not eliminate the chance for port conflict but
// reduces it significanltly with little overhead. Furthermore, asking
// the kernel for a random port by binding to port 0 prolongs the test
// execution (in our case +20sec) while also not fully eliminating the
// chance of port conflicts for concurrently executed test binaries.
// Instead of relying on one set of ports to be sufficient we retry
// starting the agent with different ports on port conflict.
func pickRandomPorts(c *Config) {
	port := FivePorts()
	c.Ports.HTTP = port + 1
	c.Ports.RPC = port + 2
	c.Ports.Serf = port + 3

	if err := c.normalizeAddrs(); err != nil {
		panic(fmt.Sprintf("error normalizing config: %v", err))
	}
}

// TestConfig returns a unique default configuration for testing an
// agent.
func (a *TestAgent) config() *Config {
	conf := DevConfig()

	// Customize the server configuration
	config := nomad.DefaultConfig()
	conf.NomadConfig = config

	// Set the name
	conf.NodeName = a.Name

	// Bind and set ports
	conf.BindAddr = "127.0.0.1"

	conf.Consul = sconfig.DefaultConsulConfig()
	conf.Vault.Enabled = new(bool)

	// Tighten the Serf timing
	config.SerfConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfConfig.MemberlistConfig.RetransmitMult = 2
	config.SerfConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	// Tighten the Raft timing
	config.RaftConfig.LeaderLeaseTimeout = 20 * time.Millisecond
	config.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	config.RaftConfig.ElectionTimeout = 40 * time.Millisecond
	config.RaftConfig.StartAsLeader = true
	config.RaftTimeout = 500 * time.Millisecond

	// Bootstrap ourselves
	config.Bootstrap = true
	config.BootstrapExpect = 1

	// Tighten the fingerprinter timeouts
	if conf.Client.Options == nil {
		conf.Client.Options = make(map[string]string)
	}
	conf.Client.Options[fingerprint.TightenNetworkTimeoutsConfig] = "true"

	if a.ConfigCallback != nil {
		a.ConfigCallback(conf)
	}

	return conf
}
