// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

const (
	vaultAvailable   = "available"
	vaultUnavailable = "unavailable"
)

// VaultFingerprint is used to fingerprint for Vault
type VaultFingerprint struct {
	logger     log.Logger
	clients    map[string]*vapi.Client
	lastStates map[string]string
}

// NewVaultFingerprint is used to create a Vault fingerprint
func NewVaultFingerprint(logger log.Logger) Fingerprint {
	return &VaultFingerprint{
		logger:     logger.Named("vault"),
		clients:    map[string]*vapi.Client{},
		lastStates: map[string]string{"default": vaultUnavailable},
	}
}

func (f *VaultFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	for _, cfg := range f.vaultConfigs(req) {
		err := f.fingerprintImpl(cfg, resp)
		if err != nil {
			return err
		}
	}
	return nil
}

// fingerprintImpl fingerprints for a single Vault cluster
func (f *VaultFingerprint) fingerprintImpl(cfg *config.VaultConfig, resp *FingerprintResponse) error {

	// Only create the client once to avoid creating too many connections to Vault
	client := f.clients[cfg.Name]
	if client == nil {
		vaultConfig, err := cfg.ApiConfig()
		if err != nil {
			return fmt.Errorf("Failed to initialize the Vault client config for %s: %v", cfg.Name, err)
		}
		client, err = vapi.NewClient(vaultConfig)
		if err != nil {
			return fmt.Errorf("Failed to initialize Vault client for %s: %s", cfg.Name, err)
		}
		f.clients[cfg.Name] = client
		useragent.SetHeaders(client)
	}

	// Connect to vault and parse its information
	status, err := client.Sys().SealStatus()
	if err != nil {
		// Print a message indicating that Vault is not available anymore
		if lastState, ok := f.lastStates[cfg.Name]; !ok || lastState == vaultAvailable {
			f.logger.Info("Vault is unavailable", "cluster", cfg.Name)
		}
		f.lastStates[cfg.Name] = vaultUnavailable
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
	if lastState, ok := f.lastStates[cfg.Name]; !ok || lastState == vaultUnavailable {
		f.logger.Info("Vault is available", "cluster", cfg.Name)
	}
	f.lastStates[cfg.Name] = vaultAvailable
	resp.Detected = true
	return nil
}

func (f *VaultFingerprint) Periodic() (bool, time.Duration) {
	for _, lastState := range f.lastStates {
		if lastState != vaultAvailable {
			return true, 15 * time.Second
		}
	}

	// Fingerprint infrequently once Vault is initially discovered with wide
	// jitter to avoid thundering herds of fingerprints against central Vault
	// servers.
	return true, (30 * time.Second) + helper.RandomStagger(90*time.Second)
}
