// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

const (
	vaultAvailable   = "available"
	vaultUnavailable = "unavailable"
)

var vaultBaseFingerprintInterval = 15 * time.Second

// VaultFingerprint is used to fingerprint for Vault
type VaultFingerprint struct {
	logger log.Logger
	states map[string]*fingerprintState
}

type fingerprintState struct {
	client      *vapi.Client
	isAvailable bool
	nextCheck   time.Time
}

// NewVaultFingerprint is used to create a Vault fingerprint
func NewVaultFingerprint(logger log.Logger) Fingerprint {
	return &VaultFingerprint{
		logger: logger.Named("vault"),
		states: map[string]*fingerprintState{},
	}
}

func (f *VaultFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	var mErr *multierror.Error
	for _, cfg := range f.vaultConfigs(req) {
		err := f.fingerprintImpl(cfg, resp)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	return mErr.ErrorOrNil()
}

// fingerprintImpl fingerprints for a single Vault cluster
func (f *VaultFingerprint) fingerprintImpl(cfg *config.VaultConfig, resp *FingerprintResponse) error {

	logger := f.logger.With("cluster", cfg.Name)

	state, ok := f.states[cfg.Name]
	if !ok {
		state = &fingerprintState{}
		f.states[cfg.Name] = state
	}
	if state.nextCheck.After(time.Now()) {
		return nil
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
		state.nextCheck = time.Time{} // always check on next interval
		return nil
	}

	if cfg.Name == "default" {
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

	// Widen the minimum window to the next check so that if one out of a set of
	// Vaults is unhealthy we don't greatly increase requests to the healthy
	// ones. This is less than the minimum window if all Vaults are healthy so
	// that we don't desync from the larger window provided by Periodic
	state.nextCheck = time.Now().Add(29 * time.Second)
	state.isAvailable = true

	resp.Detected = true

	return nil
}

func (f *VaultFingerprint) Periodic() (bool, time.Duration) {
	if len(f.states) == 0 {
		return true, vaultBaseFingerprintInterval
	}
	for _, state := range f.states {
		if !state.isAvailable {
			return true, vaultBaseFingerprintInterval
		}
	}

	// Once all Vaults are initially discovered and healthy we fingerprint with
	// a wide jitter to avoid thundering herds of fingerprints against central
	// Vault servers.
	return true, (30 * time.Second) + helper.RandomStagger(90*time.Second)
}
