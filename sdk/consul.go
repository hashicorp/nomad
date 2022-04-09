package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"oss.indeed.com/go/libtime/decay"
)

type ConsulConfig struct {
	NodeName          string       `json:"node_name"`
	LogLevel          string       `json:"log_level"`
	Ports             *PortsConfig `json:"ports"`
	Bind              string       `json:"bind_addr"`
	DataDir           string       `json:"data_dir"`
	Bootstrap         bool         `json:"bootstrap"`
	Server            bool         `json:"server"`
	Telemetry         *Telemetry   `json:"telemetry"`
	DisableCheckpoint bool         `json:"disable_update_check,omitempty"`
	Performance       *Performance `json:"performance,omitempty"`
}

type Telemetry struct {
	DisableCompatibility bool `json:"disable_compat_1.9,omitempty"`
}

type Performance struct {
	RaftMultiplier int `json:"raft_multiplier,omitempty"`
}

// Write writes the configuration JSON file of cc in a fresh temp directory to
// a file called "consul.json". The temp directory is assigned to cc.DataDir and
// is the return value.
func (cc *ConsulConfig) Write(t *testing.T) string {
	var (
		dir  string
		file string
		f    *os.File
		err  error
	)

	dir, err = ioutil.TempDir("", cc.NodeName+"-")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	cc.DataDir = dir

	file = filepath.Join(dir, "consul.json")
	f, err = os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}
	if err = json.NewEncoder(f).Encode(cc); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	if err = f.Sync(); err != nil {
		t.Fatalf("failed to sync config file: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("failed to close config file: %v", err)
	}
	return file
}

type ConsulConfigCallback func(c *ConsulConfig)

type APIConsulConfigCallback func(c *api.Config)

type Consul struct {
	Cmd    *exec.Cmd
	Config *ConsulConfig
}

func (c *Consul) HTTP() string {
	return fmt.Sprintf("http://localhost:%d", c.Config.Ports.HTTP)
}

func (c *Consul) wait(t *testing.T) {
	uri := fmt.Sprintf("http://localhost:%d/v1/status/leader", c.Config.Ports.HTTP)

	// query uri until success or eventual failure
	try := func() (retry bool, err error) {
		response, err := http.Get(uri)
		if err != nil {
			return true, err
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return true, err
		}

		// ip:port encapsulated in quotes, e.g. "192.168.10.154:8501"
		leader := strings.Trim(strings.TrimSpace(string(body)), `"`)
		port := strconv.Itoa(c.Config.Ports.RPC)
		if !strings.HasSuffix(leader, port) {
			// returns ip, not node name, just check port
			return true, errors.New("not the leader we are looking for")
		}

		return false, nil
	}

	config := decay.BackoffOptions{
		MaxSleepTime:   10 * time.Second,
		InitialGapSize: 1 * time.Second,
		MaxJitterSize:  1 * time.Second,
	}

	if err := decay.Backoff(try, config); err != nil {
		t.Fatalf("failed to acquire leader: %v", err)
	}
}

func NewConsul(t *testing.T, ccc ConsulConfigCallback) (*Consul, Wait, Stop) {
	ctx := context.Background()
	return NewConsulContext(ctx, t, ccc)
}

func NewConsulContext(ctx context.Context, t *testing.T, ccc ConsulConfigCallback) (*Consul, Wait, Stop) {
	// create initial default config
	config := DefaultConsulConfig(t)

	// apply any config modifiers
	if ccc != nil {
		ccc(config)
	}

	// write config file
	dir := config.Write(t)

	// create a Consul with Config
	c := &Consul{
		Cmd:    exec.CommandContext(ctx, "consul", "agent", "-config-dir", dir),
		Config: config,
	}

	// enable log output only if log level is not OFF
	c.Cmd.Stdout = logOutput(config.LogLevel, os.Stdout)
	c.Cmd.Stderr = logOutput(config.LogLevel, os.Stderr)

	// start the consul process
	if err := c.Cmd.Start(); err != nil {
		c.Config.Ports.Cleanup()
		_ = os.Remove(dir)
		t.Fatalf("failed to start consul: %v", err)
	}

	// create a stop function for killing and cleaning up the consul agent
	stop := func() {
		_ = c.Cmd.Process.Kill()
		_, _ = c.Cmd.Process.Wait()
		c.Config.Ports.Cleanup()
		_ = os.RemoveAll(dir)
	}

	// wait for leader
	wait := func() {
		c.wait(t)
	}

	return c, wait, stop
}

func DefaultConsulConfig(t *testing.T) *ConsulConfig {
	return &ConsulConfig{
		NodeName:          fmt.Sprintf("test-%d", nextID()),
		LogLevel:          "info",
		Ports:             FreeConsulPorts(t),
		Bind:              "127.0.0.1",
		Bootstrap:         true,
		Server:            true,
		DisableCheckpoint: true,
		Telemetry: &Telemetry{
			DisableCompatibility: true,
		},
		Performance: &Performance{
			RaftMultiplier: 1,
		},
	}
}

func (c *Consul) Client(t *testing.T) *api.Client {
	return c.ClientWithConfig(t, nil)
}

func (c *Consul) ClientWithConfig(t *testing.T, ccc APIConsulConfigCallback) *api.Client {
	config := &api.Config{
		Address:    fmt.Sprintf("localhost:%d", c.Config.Ports.HTTP),
		Scheme:     "http",
		Datacenter: "dc1",
		HttpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	if ccc != nil {
		ccc(config)
	}
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create api client: %v", err)
	}
	return client
}
