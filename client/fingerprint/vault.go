package fingerprint

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	vapi "github.com/hashicorp/vault/api"
)

const (
	vaultAvailable   = "available"
	vaultUnavailable = "unavailable"
)

// ConsulFingerprint is used to fingerprint the architecture
type VaultFingerprint struct {
	logger    *log.Logger
	client    *vapi.Client
	lastState string
}

// NewVaultFingerprint is used to create a Vault fingerprint
func NewVaultFingerprint(logger *log.Logger) Fingerprint {
	return &VaultFingerprint{logger: logger, lastState: vaultUnavailable}
}

func (f *VaultFingerprint) Fingerprint(config *client.Config, node *structs.Node) (bool, error) {
	if config.VaultConfig == nil || !config.VaultConfig.Enabled {
		return false, nil
	}

	// Only create the client once to avoid creating too many connections to
	// Vault.
	if f.client == nil {
		vaultConfig, err := config.VaultConfig.ApiConfig()
		if err != nil {
			return false, fmt.Errorf("Failed to initialize the Vault client config: %v", err)
		}

		f.client, err = vapi.NewClient(vaultConfig)
		if err != nil {
			return false, fmt.Errorf("Failed to initialize Vault client: %s", err)
		}
	}

	// Connect to vault and parse its information
	status, err := f.client.Sys().SealStatus()
	if err != nil {
		// Clear any attributes set by a previous fingerprint.
		f.clearVaultAttributes(node)

		// Print a message indicating that Vault is not available anymore
		if f.lastState == vaultAvailable {
			f.logger.Printf("[INFO] fingerprint.consul: Vault is unavailable")
		}
		f.lastState = vaultUnavailable
		return false, nil
	}

	node.Attributes["vault.accessible"] = strconv.FormatBool(true)
	// We strip the Vault prefix becasue < 0.6.2 the version looks like:
	// status.Version = "Vault v0.6.1"
	node.Attributes["vault.version"] = strings.TrimPrefix(status.Version, "Vault ")
	node.Attributes["vault.cluster_id"] = status.ClusterID
	node.Attributes["vault.cluster_name"] = status.ClusterName

	// If Vault was previously unavailable print a message to indicate the Agent
	// is available now
	if f.lastState == vaultUnavailable {
		f.logger.Printf("[INFO] fingerprint.vault: Vault is available")
	}
	f.lastState = vaultAvailable
	return true, nil
}

func (f *VaultFingerprint) clearVaultAttributes(n *structs.Node) {
	delete(n.Attributes, "vault.accessible")
	delete(n.Attributes, "vault.version")
	delete(n.Attributes, "vault.cluster_id")
	delete(n.Attributes, "vault.cluster_name")
}

func (f *VaultFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}
