package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

// TestVault is a test helper. It uses a fork/exec model to create a test Vault
// server instance in the background and can be initialized with policies, roles
// and backends mounted. The test Vault instances can be used to run a unit test
// and offers and easy API to tear itself down on test end. The only
// prerequisite is that the Vault binary is on the $PATH.

const (
	// vaultStartPort is the starting port we use to bind Vault servers to
	vaultStartPort uint64 = 40000
)

// vaultPortOffset is used to atomically increment the port numbers.
var vaultPortOffset uint64

// TestVault wraps a test Vault server launched in dev mode, suitable for
// testing.
type TestVault struct {
	cmd *exec.Cmd
	t   *testing.T

	Addr      string
	HTTPAddr  string
	RootToken string
	Config    *config.VaultConfig
	Client    *vapi.Client
}

// NewTestVault returns a new TestVault instance that has yet to be started
func NewTestVault(t *testing.T) *TestVault {
	port := getPort()
	token := structs.GenerateUUID()
	bind := fmt.Sprintf("-dev-listen-address=127.0.0.1:%d", port)
	http := fmt.Sprintf("http://127.0.0.1:%d", port)
	root := fmt.Sprintf("-dev-root-token-id=%s", token)

	cmd := exec.Command("vault", "server", "-dev", bind, root)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Build the config
	conf := vapi.DefaultConfig()
	conf.Address = http

	// Make the client and set the token to the root token
	client, err := vapi.NewClient(conf)
	if err != nil {
		t.Fatalf("failed to build Vault API client: %v", err)
	}
	client.SetToken(token)

	enable := true
	tv := &TestVault{
		cmd:       cmd,
		t:         t,
		Addr:      bind,
		HTTPAddr:  http,
		RootToken: token,
		Client:    client,
		Config: &config.VaultConfig{
			Enabled: &enable,
			Token:   token,
			Addr:    http,
		},
	}

	return tv
}

// Start starts the test Vault server and waits for it to respond to its HTTP
// API
func (tv *TestVault) Start() *TestVault {
	if err := tv.cmd.Start(); err != nil {
		tv.t.Fatalf("failed to start vault: %v", err)
	}

	tv.waitForAPI()
	return tv
}

// Stop stops the test Vault server
func (tv *TestVault) Stop() {
	if tv.cmd.Process == nil {
		return
	}

	if err := tv.cmd.Process.Kill(); err != nil {
		tv.t.Errorf("err: %s", err)
	}
	tv.cmd.Wait()
}

// waitForAPI waits for the Vault HTTP endpoint to start
// responding. This is an indication that the agent has started.
func (tv *TestVault) waitForAPI() {
	WaitForResult(func() (bool, error) {
		inited, err := tv.Client.Sys().InitStatus()
		if err != nil {
			return false, err
		}
		return inited, nil
	}, func(err error) {
		defer tv.Stop()
		tv.t.Fatalf("err: %s", err)
	})
}

// getPort returns the next available port to bind Vault against
func getPort() uint64 {
	p := vaultStartPort + vaultPortOffset
	vaultPortOffset += 1
	return p
}
