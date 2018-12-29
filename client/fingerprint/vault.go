package fingerprint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	vapi "github.com/hashicorp/vault/api"
)

const (
	vaultAvailable   = "available"
	vaultUnavailable = "unavailable"
)

// VaultFingerprint is used to fingerprint for Vault
type VaultFingerprint struct {
	logger    log.Logger
	client    *vapi.Client
	lastState string
}

// NewVaultFingerprint is used to create a Vault fingerprint
func NewVaultFingerprint(logger log.Logger) Fingerprint {
	return &VaultFingerprint{logger: logger.Named("vault"), lastState: vaultUnavailable}
}

func (f *VaultFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	config := req.Config

	if config.VaultConfig == nil || !config.VaultConfig.IsEnabled() {
		return nil
	}

	// Only create the client once to avoid creating too many connections to
	// Vault.
	if f.client == nil {
		vaultConfig, err := config.VaultConfig.ApiConfig()
		if err != nil {
			return fmt.Errorf("Failed to initialize the Vault client config: %v", err)
		}

		f.client, err = vapi.NewClient(vaultConfig)
		if err != nil {
			return fmt.Errorf("Failed to initialize Vault client: %s", err)
		}
	}

	// Connect to vault and parse its information
	status, err := f.client.Sys().SealStatus()
	if err != nil {
		f.clearVaultAttributes(resp)
		// Print a message indicating that Vault is not available anymore
		if f.lastState == vaultAvailable {
			f.logger.Info("Vault is unavailable")
		}
		f.lastState = vaultUnavailable
		return nil
	}

	resp.AddAttribute("vault.accessible", strconv.FormatBool(true))
	// We strip the Vault prefix because < 0.6.2 the version looks like:
	// status.Version = "Vault v0.6.1"
	resp.AddAttribute("vault.version", strings.TrimPrefix(status.Version, "Vault "))
	resp.AddAttribute("vault.cluster_id", status.ClusterID)
	resp.AddAttribute("vault.cluster_name", status.ClusterName)

	// If Vault was previously unavailable print a message to indicate the Agent
	// is available now
	if f.lastState == vaultUnavailable {
		f.logger.Info("Vault is available")
	}
	f.lastState = vaultAvailable
	resp.Detected = true
	return nil
}

func (f *VaultFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

func (f *VaultFingerprint) clearVaultAttributes(r *FingerprintResponse) {
	r.RemoveAttribute("vault.accessible")
	r.RemoveAttribute("vault.version")
	r.RemoveAttribute("vault.cluster_id")
	r.RemoveAttribute("vault.cluster_name")
}
