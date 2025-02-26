// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

// VaultFingerprint is used to fingerprint for Vault
type VaultFingerprint struct {
	logger log.Logger
	states map[string]*vaultFingerprintState

	// Once initial fingerprints are complete, we no-op all periodic
	// fingerprints to prevent Vault availability issues causing a thundering
	// herd of node updates. This behavior resets if we reload the
	// configuration.
	initialResponse     *FingerprintResponse
	initialResponseLock sync.RWMutex
}

type vaultFingerprintState struct {
	client            *vapi.Client
	isAvailable       bool
	fingerprintedOnce bool
}

// NewVaultFingerprint is used to create a Vault fingerprint
func NewVaultFingerprint(logger log.Logger) Fingerprint {
	return &VaultFingerprint{
		logger: logger.Named("vault"),
		states: map[string]*vaultFingerprintState{},
	}
}

func (f *VaultFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	if f.readInitialResponse(resp) {
		return nil
	}
	var mErr *multierror.Error
	vaultConfigs := req.Config.GetVaultConfigs(f.logger)

	for _, cfg := range vaultConfigs {
		err := f.fingerprintImpl(cfg, resp)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	fingerprintCount := 0
	for _, state := range f.states {
		if state.fingerprintedOnce {
			fingerprintCount++
		}
	}
	if fingerprintCount == len(vaultConfigs) {
		f.setInitialResponse(resp)
	}

	return mErr.ErrorOrNil()
}

// readInitialResponse checks for a previously seen response. It returns true
// and shallow-copies the response into the argument if one is available. We
// only want to hold the lock open during the read and not the Fingerprint so
// that we don't block a Reload call while waiting for Vault requests to
// complete. If the Reload clears the initialResponse after we take the lock
// again in setInitialResponse (ex. 2 reloads quickly in a row), the worst that
// happens is we do an extra fingerprint when the Reload caller calls
// Fingerprint
func (f *VaultFingerprint) readInitialResponse(resp *FingerprintResponse) bool {
	f.initialResponseLock.RLock()
	defer f.initialResponseLock.RUnlock()
	if f.initialResponse != nil {
		*resp = *f.initialResponse
		return true
	}
	return false
}

func (f *VaultFingerprint) setInitialResponse(resp *FingerprintResponse) {
	f.initialResponseLock.Lock()
	defer f.initialResponseLock.Unlock()
	f.initialResponse = resp
}

// fingerprintImpl fingerprints for a single Vault cluster
func (f *VaultFingerprint) fingerprintImpl(cfg *config.VaultConfig, resp *FingerprintResponse) error {
	logger := f.logger.With("cluster", cfg.Name)

	state, ok := f.states[cfg.Name]
	if !ok {
		state = &vaultFingerprintState{}
		f.states[cfg.Name] = state
	}

	// Only create the client once to avoid creating too many connections to Vault
	if state.client == nil {
		vaultConfig, err := cfg.ApiConfig()
		if err != nil {
			return fmt.Errorf("Failed to initialize the Vault client config for %s: %v", cfg.Name, err)
		}
		state.client, err = vapi.NewClient(vaultConfig)
		if err != nil {
			return fmt.Errorf("Failed to initialize Vault client for %s: %s", cfg.Name, err)
		}
		useragent.SetHeaders(state.client)
	}

	// Connect to vault and parse its information
	status, err := state.client.Sys().SealStatus()
	if err != nil {
		// Print a message indicating that Vault is not available anymore
		if state.isAvailable {
			logger.Info("Vault is unavailable")
		}
		state.isAvailable = false
		return nil
	}

	if cfg.Name == structs.VaultDefaultCluster {
		resp.AddAttribute("vault.accessible", strconv.FormatBool(true))
		resp.AddAttribute("vault.version", strings.TrimPrefix(status.Version, "Vault "))
		resp.AddAttribute("vault.cluster_id", status.ClusterID)
		resp.AddAttribute("vault.cluster_name", status.ClusterName)
	} else {
		resp.AddAttribute(fmt.Sprintf("vault.%s.accessible", cfg.Name), strconv.FormatBool(true))
		resp.AddAttribute(fmt.Sprintf("vault.%s.version", cfg.Name), strings.TrimPrefix(status.Version, "Vault "))
		resp.AddAttribute(fmt.Sprintf("vault.%s.cluster_id", cfg.Name), status.ClusterID)
		resp.AddAttribute(fmt.Sprintf("vault.%s.cluster_name", cfg.Name), status.ClusterName)
	}

	// If Vault was previously unavailable print a message to indicate the Agent
	// is available now
	if !state.isAvailable {
		logger.Info("Vault is available")
	}

	state.isAvailable = true
	state.fingerprintedOnce = true
	resp.Detected = true

	return nil
}

func (f *VaultFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

// Reload satisfies ReloadableFingerprint and resets the gate on periodic fingerprinting.
func (f *VaultFingerprint) Reload() {
	f.setInitialResponse(nil)
}
