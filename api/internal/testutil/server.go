// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

// TestServer is a test helper. It uses a fork/exec model to create
// a test Nomad server instance in the background and initialize it
// with some data and/or services. The test server can then be used
// to run a unit test, and offers an easy API to tear itself down
// when the test has completed. The only prerequisite is to have a nomad
// binary available on the $PATH.
//
// This package does not use Nomad's official API client. This is
// because we use TestServer to test the API client, which would
// otherwise cause an import cycle.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/api/internal/testutil/discover"
	testing "github.com/mitchellh/go-testing-interface"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// TestServerConfig is the main server configuration struct.
type TestServerConfig struct {
	NodeName          string        `json:"name,omitempty"`
	DataDir           string        `json:"data_dir,omitempty"`
	Region            string        `json:"region,omitempty"`
	DisableCheckpoint bool          `json:"disable_update_check"`
	LogLevel          string        `json:"log_level,omitempty"`
	Consul            *Consul       `json:"consul,omitempty"`
	AdvertiseAddrs    *Advertise    `json:"advertise,omitempty"`
	Ports             *PortsConfig  `json:"ports,omitempty"`
	Server            *ServerConfig `json:"server,omitempty"`
	Client            *ClientConfig `json:"client,omitempty"`
	Vault             *VaultConfig  `json:"vault,omitempty"`
	ACL               *ACLConfig    `json:"acl,omitempty"`
	Telemetry         *Telemetry    `json:"telemetry,omitempty"`
	DevMode           bool          `json:"-"`
	Stdout, Stderr    io.Writer     `json:"-"`
}

// Consul is used to configure the communication with Consul
type Consul struct {
	Address string `json:"address,omitempty"`
	Auth    string `json:"auth,omitempty"`
	Token   string `json:"token,omitempty"`
}

// Advertise is used to configure the addresses to advertise
type Advertise struct {
	HTTP string `json:"http,omitempty"`
	RPC  string `json:"rpc,omitempty"`
	Serf string `json:"serf,omitempty"`
}

// PortsConfig is used to configure the network ports we use.
type PortsConfig struct {
	HTTP int `json:"http,omitempty"`
	RPC  int `json:"rpc,omitempty"`
	Serf int `json:"serf,omitempty"`
}

// ServerConfig is used to configure the nomad server.
type ServerConfig struct {
	Enabled         bool `json:"enabled"`
	BootstrapExpect int  `json:"bootstrap_expect"`
	RaftProtocol    int  `json:"raft_protocol,omitempty"`
}

// ClientConfig is used to configure the client
type ClientConfig struct {
	Enabled bool              `json:"enabled"`
	Options map[string]string `json:"options,omitempty"`
}

// VaultConfig is used to configure Vault
type VaultConfig struct {
	Enabled bool `json:"enabled"`
}

// ACLConfig is used to configure ACLs
type ACLConfig struct {
	Enabled bool `json:"enabled"`
}

// Telemetry is used to configure the Nomad telemetry setup.
type Telemetry struct {
	PrometheusMetrics bool `json:"prometheus_metrics"`
}

// ServerConfigCallback is a function interface which can be
// passed to NewTestServerConfig to modify the server config.
type ServerConfigCallback func(c *TestServerConfig)

// defaultServerConfig returns a new TestServerConfig struct pre-populated with
// usable config for running as server.
func defaultServerConfig(t testing.T) *TestServerConfig {
	ports := PortAllocator.Grab(3)

	logLevel := "ERROR"
	if envLogLevel := os.Getenv("NOMAD_TEST_LOG_LEVEL"); envLogLevel != "" {
		logLevel = envLogLevel
	}

	return &TestServerConfig{
		NodeName:          fmt.Sprintf("node-%d", ports[0]),
		DisableCheckpoint: true,
		LogLevel:          logLevel,
		Ports: &PortsConfig{
			HTTP: ports[0],
			RPC:  ports[1],
			Serf: ports[2],
		},
		Server: &ServerConfig{
			Enabled:         true,
			BootstrapExpect: 1,
		},
		Client: &ClientConfig{
			Enabled: false,
		},
		Vault: &VaultConfig{
			Enabled: false,
		},
		ACL: &ACLConfig{
			Enabled: false,
		},
	}
}

// TestServer is the main server wrapper struct.
type TestServer struct {
	cmd    *exec.Cmd
	Config *TestServerConfig
	t      testing.T

	HTTPAddr   string
	SerfAddr   string
	HTTPClient *http.Client
}

// NewTestServer creates a new TestServer, and makes a call to
// an optional callback function to modify the configuration.
func NewTestServer(t testing.T, cb ServerConfigCallback) *TestServer {
	path, err := discover.NomadExecutable()
	if err != nil {
		t.Skipf("nomad not found, skipping: %v", err)
	}

	// Check that we are actually running nomad
	_, err = exec.Command(path, "-version").CombinedOutput()
	must.NoError(t, err)

	dataDir, err := os.MkdirTemp("", "nomad")
	must.NoError(t, err)

	configFile, err := os.CreateTemp(dataDir, "nomad")
	must.NoError(t, err)

	nomadConfig := defaultServerConfig(t)
	nomadConfig.DataDir = dataDir

	if cb != nil {
		cb(nomadConfig)
	}

	if nomadConfig.DevMode {
		if nomadConfig.Client.Options == nil {
			nomadConfig.Client.Options = map[string]string{}
		}
		nomadConfig.Client.Options["test.tighten_network_timeouts"] = "true"
	}

	configContent, err := json.Marshal(nomadConfig)
	must.NoError(t, err)

	_, err = configFile.Write(configContent)
	must.NoError(t, err)
	must.NoError(t, configFile.Sync())
	must.NoError(t, configFile.Close())

	args := []string{"agent", "-config", configFile.Name()}
	if nomadConfig.DevMode {
		args = append(args, "-dev")
	}

	stdout := io.Writer(os.Stdout)
	if nomadConfig.Stdout != nil {
		stdout = nomadConfig.Stdout
	}

	stderr := io.Writer(os.Stderr)
	if nomadConfig.Stderr != nil {
		stderr = nomadConfig.Stderr
	}

	// Start the server
	cmd := exec.Command(path, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	must.NoError(t, cmd.Start())

	client := cleanhttp.DefaultClient()
	client.Timeout = 10 * time.Second

	server := &TestServer{
		Config: nomadConfig,
		cmd:    cmd,
		t:      t,

		HTTPAddr:   fmt.Sprintf("127.0.0.1:%d", nomadConfig.Ports.HTTP),
		SerfAddr:   fmt.Sprintf("127.0.0.1:%d", nomadConfig.Ports.Serf),
		HTTPClient: client,
	}

	// Wait for the server to be ready
	if nomadConfig.Server.Enabled && nomadConfig.Server.BootstrapExpect != 0 {
		server.waitForLeader()
	} else {
		server.waitForAPI()
	}

	// Wait for the client to be ready
	if nomadConfig.DevMode {
		server.waitForClient()
	}
	return server
}

// Stop stops the test Nomad server, and removes the Nomad data
// directory once we are done.
func (s *TestServer) Stop() {
	defer func() { _ = os.RemoveAll(s.Config.DataDir) }()

	// wait for the process to exit to be sure that the data dir can be
	// deleted on all platforms.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.cmd.Wait()
	}()

	// kill and wait gracefully
	err := s.cmd.Process.Signal(os.Interrupt)
	must.NoError(s.t, err)

	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
		s.t.Logf("timed out waiting for process to gracefully terminate")
	}

	err = s.cmd.Process.Kill()
	must.NoError(s.t, err, must.Sprint("failed to kill process"))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.t.Logf("timed out waiting for process to be killed")
	}
}

// waitForAPI waits for only the agent HTTP endpoint to start
// responding. This is an indication that the agent has started,
// but will likely return before a leader is elected.
func (s *TestServer) waitForAPI() {
	f := func() error {
		resp, err := s.HTTPClient.Get(s.url("/v1/metrics"))
		if err != nil {
			return fmt.Errorf("failed to get metrics: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if err = s.requireOK(resp); err != nil {
			return fmt.Errorf("metrics response is not ok: %w", err)
		}
		return nil
	}
	must.Wait(s.t,
		wait.InitialSuccess(
			wait.ErrorFunc(f),
			wait.Timeout(10*time.Second),
			wait.Gap(1*time.Second),
		),
		must.Sprint("failed to wait for api"),
	)
}

// waitForLeader waits for the Nomad server's HTTP API to become available, and
// then waits for the keyring to be intialized. This implies a leader has been
// elected and Raft writes have occurred.
func (s *TestServer) waitForLeader() {
	f := func() error {
		resp, err := s.HTTPClient.Get(s.url("/.well-known/jwks.json"))
		if err != nil {
			return fmt.Errorf("failed to contact leader: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if err = s.requireOK(resp); err != nil {
			return fmt.Errorf("leader response is not ok: %w", err)
		}

		jwks := struct {
			Keys []interface{} `json:"keys"`
		}{}
		if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
			return fmt.Errorf("error decoding jwks response: %w", err)
		}
		if len(jwks.Keys) == 0 {
			return fmt.Errorf("no keys found")
		}
		return nil
	}
	must.Wait(s.t,
		wait.InitialSuccess(
			wait.ErrorFunc(f),
			wait.Timeout(10*time.Second),
			wait.Gap(1*time.Second),
		),
		must.Sprint("failed to wait for leader"),
	)
}

// waitForClient waits for the Nomad client to be ready. The function returns
// immediately if the server is not in dev mode.
func (s *TestServer) waitForClient() {
	if !s.Config.DevMode {
		return
	}
	f := func() error {
		resp, err := s.HTTPClient.Get(s.url("/v1/nodes"))
		if err != nil {
			return fmt.Errorf("failed to get nodes: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if err = s.requireOK(resp); err != nil {
			return fmt.Errorf("nodes response not ok: %w", err)
		}
		var decoded []struct {
			ID     string
			Status string
		}
		if err = json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
			return fmt.Errorf("failed to decode nodes response: %w", err)
		}
		return nil
	}
	must.Wait(s.t,
		wait.InitialSuccess(
			wait.ErrorFunc(f),
			wait.Timeout(10*time.Second),
			wait.Gap(1*time.Second),
		),
		must.Sprint("failed to wait for client (node)"),
	)
}

// url is a helper function which takes a relative URL and
// makes it into a proper URL against the local Nomad server.
func (s *TestServer) url(path string) string {
	return fmt.Sprintf("http://%s%s", s.HTTPAddr, path)
}

// requireOK checks the HTTP response code and ensures it is acceptable.
func (s *TestServer) requireOK(resp *http.Response) error {
	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	return nil
}

// put performs a new HTTP PUT request.
func (s *TestServer) put(path string, body io.Reader) *http.Response {
	req, err := http.NewRequest("PUT", s.url(path), body)
	must.NoError(s.t, err)

	resp, err := s.HTTPClient.Do(req)
	must.NoError(s.t, err)

	if err = s.requireOK(resp); err != nil {
		_ = resp.Body.Close()
		must.NoError(s.t, err)
	}
	return resp
}

// get performs a new HTTP GET request.
func (s *TestServer) get(path string) *http.Response {
	resp, err := s.HTTPClient.Get(s.url(path))
	must.NoError(s.t, err)

	if err = s.requireOK(resp); err != nil {
		_ = resp.Body.Close()
		must.NoError(s.t, err)
	}
	return resp
}

// encodePayload returns a new io.Reader wrapping the encoded contents
// of the payload, suitable for passing directly to a new request.
func (s *TestServer) encodePayload(payload any) io.Reader {
	var encoded bytes.Buffer
	err := json.NewEncoder(&encoded).Encode(payload)
	must.NoError(s.t, err)
	return &encoded
}
