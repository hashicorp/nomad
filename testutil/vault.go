package testutil

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-testing-interface"
)

// TestVault is a test helper. It uses a fork/exec model to create a test Vault
// server instance in the background and can be initialized with policies, roles
// and backends mounted. The test Vault instances can be used to run a unit test
// and offers and easy API to tear itself down on test end. The only
// prerequisite is that the Vault binary is on the $PATH.

// TestVault wraps a test Vault server launched in dev mode, suitable for
// testing.
type TestVault struct {
	cmd    *exec.Cmd
	t      testing.T
	waitCh chan error

	Addr      string
	HTTPAddr  string
	RootToken string
	Config    *config.VaultConfig
	Client    *vapi.Client
}

// NewTestVault returns a new TestVault instance that has yet to be started
func NewTestVault(t testing.T) *TestVault {
	for i := 10; i >= 0; i-- {
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

		if err := tv.cmd.Start(); err != nil {
			tv.t.Fatalf("failed to start vault: %v", err)
		}

		// Start the waiter
		tv.waitCh = make(chan error, 1)
		go func() {
			err := tv.cmd.Wait()
			tv.waitCh <- err
		}()

		// Ensure Vault started
		var startErr error
		select {
		case startErr = <-tv.waitCh:
		case <-time.After(time.Duration(500*TestMultiplier()) * time.Millisecond):
		}

		if startErr != nil && i == 0 {
			t.Fatalf("failed to start vault: %v", startErr)
		} else if startErr != nil {
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
			continue
		}

		waitErr := tv.waitForAPI()
		if waitErr != nil && i == 0 {
			t.Fatalf("failed to start vault: %v", waitErr)
		} else if waitErr != nil {
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
			continue
		}

		return tv
	}

	return nil
}

// NewTestVaultDelayed returns a test Vault server that has not been started.
// Start must be called and it is the callers responsibility to deal with any
// port conflicts that may occur and retry accordingly.
func NewTestVaultDelayed(t testing.T) *TestVault {
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
func (tv *TestVault) Start() error {
	if err := tv.cmd.Start(); err != nil {
		tv.t.Fatalf("failed to start vault: %v", err)
	}

	// Start the waiter
	tv.waitCh = make(chan error, 1)
	go func() {
		err := tv.cmd.Wait()
		tv.waitCh <- err
	}()

	// Ensure Vault started
	select {
	case err := <-tv.waitCh:
		return err
	case <-time.After(time.Duration(500*TestMultiplier()) * time.Millisecond):
	}

	return tv.waitForAPI()
}

// Stop stops the test Vault server
func (tv *TestVault) Stop() {
	if tv.cmd.Process == nil {
		return
	}

	if err := tv.cmd.Process.Kill(); err != nil {
		tv.t.Errorf("err: %s", err)
	}
	if tv.waitCh != nil {
		<-tv.waitCh
	}
}

// waitForAPI waits for the Vault HTTP endpoint to start
// responding. This is an indication that the agent has started.
func (tv *TestVault) waitForAPI() error {
	var waitErr error
	WaitForResult(func() (bool, error) {
		inited, err := tv.Client.Sys().InitStatus()
		if err != nil {
			return false, err
		}
		return inited, nil
	}, func(err error) {
		waitErr = err
	})
	return waitErr
}

func getPort() int {
	return 1030 + int(rand.Int31n(6440))
}

// VaultVersion returns the Vault version as a string or an error if it couldn't
// be determined
func VaultVersion() (string, error) {
	cmd := exec.Command("vault", "version")
	out, err := cmd.Output()
	return string(out), err
}
