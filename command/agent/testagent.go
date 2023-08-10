// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
)

// TempDir defines the base dir for temporary directories.
var TempDir = os.TempDir()

// TestAgent encapsulates an Agent with a default configuration and startup
// procedure suitable for testing. It manages a temporary data directory which
// is removed after shutdown.
type TestAgent struct {
	// T is the testing object
	T testing.TB

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

	// logger is used for logging
	logger hclog.InterceptLogger

	// DataDir is the data directory which is used when Config.DataDir
	// is not set. It is created automatically and removed when
	// Shutdown() is called.
	DataDir string

	// Key is the optional encryption key for the keyring.
	Key string

	// All HTTP servers started. Used to prevent server leaks and preserve
	// backwards compatibility.
	Servers []*HTTPServer

	// Server is a reference to the primary, started HTTP endpoint.
	// It is valid after Start().
	Server *HTTPServer

	// Agent is the embedded Nomad agent.
	// It is valid after Start().
	*Agent

	// RootToken is auto-bootstrapped if ACLs are enabled
	RootToken *structs.ACLToken

	// ports that are reserved through freeport that must be returned at
	// the end of a test, done when Shutdown() is called.
	ports []int

	// Enterprise specifies if the agent is enterprise or not
	Enterprise bool

	// shutdown is set to true if agent has been shutdown
	shutdown bool
}

// NewTestAgent returns a started agent with the given name and
// configuration. The caller should call Shutdown() to stop the agent and
// remove temporary directories.
func NewTestAgent(t testing.TB, name string, configCallback func(*Config)) *TestAgent {
	logger := testlog.HCLogger(t)
	logger.SetLevel(testlog.HCLoggerTestLevel())
	a := &TestAgent{
		T:              t,
		Name:           name,
		ConfigCallback: configCallback,
		Enterprise:     EnterpriseTestAgent,
		logger:         logger,
	}
	a.Start()
	return a
}

// Start starts a test agent.
func (a *TestAgent) Start() *TestAgent {
	if a.Agent != nil {
		a.T.Fatalf("TestAgent already started")
	}
	if a.Config == nil {
		a.Config = a.config()
	}
	defaultEnterpriseTestServerConfig(a.Config.Server)

	if a.Config.DataDir == "" {
		name := "agent"
		if a.Name != "" {
			name = a.Name + "-agent"
		}
		name = strings.ReplaceAll(name, "/", "_")
		d, err := os.MkdirTemp(TempDir, name)
		if err != nil {
			a.T.Fatalf("Error creating data dir %s: %s", filepath.Join(TempDir, name), err)
		}
		a.DataDir = d
		a.Config.DataDir = d
		a.Config.NomadConfig.DataDir = d
	}

	i := 10

	advertiseAddrs := *a.Config.AdvertiseAddrs
RETRY:
	i--

	// Clear out the advertise addresses such that through retries we
	// re-normalize the addresses correctly instead of using the values from the
	// last port selection that had a port conflict.
	newAddrs := advertiseAddrs
	a.Config.AdvertiseAddrs = &newAddrs
	a.pickRandomPorts(a.Config)
	if a.Config.NodeName == "" {
		a.Config.NodeName = fmt.Sprintf("Node %d", a.Config.Ports.RPC)
	}

	// Create a null logger before initializing the keyring. This is typically
	// done using the agent's logger. However, it hasn't been created yet.
	logger := hclog.NewNullLogger()

	// write the keyring
	if a.Key != "" {
		writeKey := func(key, filename string) {
			path := filepath.Join(a.Config.DataDir, filename)
			if err := initKeyring(path, key, logger); err != nil {
				a.T.Fatalf("Error creating keyring %s: %s", path, err)
			}
		}
		writeKey(a.Key, serfKeyring)
	}

	// we need the err var in the next exit condition
	agent, err := a.start()
	if err == nil {
		a.Agent = agent
	} else if i == 0 {
		a.T.Fatalf("%s: Error starting agent: %v", a.Name, err)
	} else {

		if agent != nil {
			agent.Shutdown()
		}
		wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
		a.T.Logf("%s: retrying in %v", a.Name, wait)
		time.Sleep(wait)

		// Clean out the data dir if we are responsible for it before we
		// try again, since the old ports may have gotten written to
		// the data dir, such as in the Raft configuration.
		if a.DataDir != "" {
			if err := os.RemoveAll(a.DataDir); err != nil {
				a.T.Fatalf("%s: Error resetting data dir: %v", a.Name, err)
			}
		}

		goto RETRY
	}

	failed := false
	if a.Config.NomadConfig.BootstrapExpect == 1 && a.Config.Server.Enabled {
		testutil.WaitForResult(func() (bool, error) {
			args := &structs.GenericRequest{}
			var leader string
			err := a.RPC("Status.Leader", args, &leader)
			return leader != "", err
		}, func(err error) {
			a.T.Logf("failed to find leader: %v", err)
			failed = true
		})
	} else {
		testutil.WaitForResult(func() (bool, error) {
			req, _ := http.NewRequest(http.MethodGet, "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			_, err := a.Server.AgentSelfRequest(resp, req)
			return err == nil && resp.Code == 200, err
		}, func(err error) {
			a.T.Logf("failed to find leader: %v", err)
			failed = true
		})
	}
	if failed {
		a.Agent.Shutdown()
		if i == 0 {
			a.T.Fatalf("ran out of retries trying to start test agent")
		}
		goto RETRY
	}

	// Check if ACLs enabled. Use special value of PolicyTTL 0s
	// to do a bypass of this step. This is so we can test bootstrap
	// without having to pass down a special flag.
	if a.Config.ACL.Enabled && a.Config.Server.Enabled && a.Config.ACL.PolicyTTL != 0 {
		a.RootToken = mock.ACLManagementToken()
		state := a.Agent.server.State()
		if err := state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, a.RootToken); err != nil {
			a.T.Fatalf("token bootstrap failed: %v", err)
		}
	}
	return a
}

func (a *TestAgent) start() (*Agent, error) {
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.NewGlobal(metrics.DefaultConfig("service-name"), inm)

	if inm == nil {
		return nil, fmt.Errorf("unable to set up in memory metrics needed for agent initialization")
	}

	agent, err := NewAgent(a.Config, a.logger, testlog.NewWriter(a.T), inm)
	if err != nil {
		return nil, err
	}

	// Setup the HTTP server
	httpServers, err := NewHTTPServers(agent, a.Config)
	if err != nil {
		return agent, err
	}

	// TODO: investigate if there is a way to remove the requirement by updating test.
	// Initial pass at implementing this is https://github.com/kevinschoonover/nomad/tree/tests.
	a.Servers = httpServers
	a.Server = httpServers[0]
	return agent, nil
}

// Shutdown stops the agent and removes the data directory if it is
// managed by the test agent.
func (a *TestAgent) Shutdown() {
	if a == nil || a.shutdown {
		return
	}
	a.shutdown = true

	defer func() {
		if a.DataDir != "" {
			_ = os.RemoveAll(a.DataDir)
		}
	}()

	// shutdown agent before endpoints
	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		for _, srv := range a.Servers {
			srv.Shutdown()
		}

		ch <- a.Agent.Shutdown()
	}()

	// one minute grace period on shutdown
	timer, cancel := helper.NewSafeTimer(1 * time.Minute)
	defer cancel()

	select {
	case err := <-ch:
		if err != nil {
			a.T.Fatalf("agent shutdown error: %v", err)
		}
	case <-timer.C:
		a.T.Fatal("agent shutdown timeout")
	}
}

func (a *TestAgent) HTTPAddr() string {
	if a.Server == nil {
		return ""
	}
	proto := "http://"
	if a.Config.TLSConfig != nil && a.Config.TLSConfig.EnableHTTP {
		proto = "https://"
	}
	return proto + a.Server.Addr
}

func (a *TestAgent) Client() *api.Client {
	conf := api.DefaultConfig()
	conf.Address = a.HTTPAddr()
	c, err := api.NewClient(conf)
	if err != nil {
		a.T.Fatalf("Error creating Nomad API client: %s", err)
	}
	return c
}

// pickRandomPorts selects random ports from fixed size random blocks of
// ports. This does not eliminate the chance for port conflict but
// reduces it significantly with little overhead. Furthermore, asking
// the kernel for a random port by binding to port 0 prolongs the test
// execution (in our case +20sec) while also not fully eliminating the
// chance of port conflicts for concurrently executed test binaries.
// Instead of relying on one set of ports to be sufficient we retry
// starting the agent with different ports on port conflict.
func (a *TestAgent) pickRandomPorts(c *Config) {
	ports := ci.PortAllocator.Grab(3)
	a.ports = append(a.ports, ports...)

	c.Ports.HTTP = ports[0]
	c.Ports.RPC = ports[1]
	c.Ports.Serf = ports[2]

	if err := c.normalizeAddrs(); err != nil {
		a.T.Fatalf("error normalizing config: %v", err)
	}
}

// TestConfig returns a unique default configuration for testing an agent.
func (a *TestAgent) config() *Config {
	conf := DevConfig(nil)
	conf.Version.BuildDate = time.Now()

	// Customize the server configuration
	config := nomad.DefaultConfig()
	conf.NomadConfig = config

	// Setup client config
	conf.ClientConfig = client.DefaultConfig()

	conf.LogLevel = testlog.HCLoggerTestLevel().String()
	conf.NomadConfig.Logger = a.logger
	conf.ClientConfig.Logger = a.logger

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
	config.RaftTimeout = 500 * time.Millisecond

	// Tighten the autopilot timing
	config.AutopilotConfig.ServerStabilizationTime = 100 * time.Millisecond
	config.ServerHealthInterval = 50 * time.Millisecond
	config.AutopilotInterval = 100 * time.Millisecond

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
