package nomad

import (
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
)

func TestVaultClient_BadConfig(t *testing.T) {
	conf := &config.VaultConfig{}
	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Should be no error since Vault is not enabled
	client, err := NewVaultClient(conf, logger)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if client.ConnectionEstablished() {
		t.Fatalf("bad")
	}

	conf.Enabled = true
	_, err = NewVaultClient(conf, logger)
	if err == nil || !strings.Contains(err.Error(), "token must be set") {
		t.Fatalf("Expected token unset error: %v", err)
	}

	conf.Token = "123"
	_, err = NewVaultClient(conf, logger)
	if err == nil || !strings.Contains(err.Error(), "address must be set") {
		t.Fatalf("Expected address unset error: %v", err)
	}
}

// Test that the Vault Client can establish a connection even if it is started
// before Vault is available.
func TestVaultClient_EstablishConnection(t *testing.T) {
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	client, err := NewVaultClient(v.Config, logger)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}

	// Sleep a little while and check that no connection has been established.
	time.Sleep(100 * time.Duration(testutil.TestMultiplier()) * time.Millisecond)

	if client.ConnectionEstablished() {
		t.Fatalf("ConnectionEstablished() returned true before Vault server started")
	}

	// Start Vault
	v.Start()

	testutil.WaitForResult(func() (bool, error) {
		return client.ConnectionEstablished(), nil
	}, func(err error) {
		t.Fatalf("Connection not established")
	})

	// Ensure that since we are using a root token that we haven started the
	// renewal loop.
	if client.renewalRunning {
		t.Fatalf("No renewal loop should be running")
	}
}

func TestVaultClient_RenewalLoop(t *testing.T) {
	v := testutil.NewTestVault(t).Start()
	defer v.Stop()

}
