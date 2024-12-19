// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

type NoopVault struct {
	l sync.Mutex

	config  *config.VaultConfig
	logger  log.Logger
	purgeFn PurgeVaultAccessorFn
}

func NewNoopVault(c *config.VaultConfig, logger log.Logger, purgeFn PurgeVaultAccessorFn) *NoopVault {
	return &NoopVault{
		config:  c,
		logger:  logger.Named("vault-noop"),
		purgeFn: purgeFn,
	}
}

func (v *NoopVault) SetActive(_ bool) {}

func (v *NoopVault) SetConfig(c *config.VaultConfig) error {
	v.l.Lock()
	defer v.l.Unlock()

	v.config = c
	return nil
}

func (v *NoopVault) GetConfig() *config.VaultConfig {
	v.l.Lock()
	defer v.l.Unlock()

	return v.config.Copy()
}

func (v *NoopVault) CreateToken(_ context.Context, _ *structs.Allocation, _ string) (*vapi.Secret, error) {
	return nil, errors.New("Nomad server is not configured to create tokens")
}

func (v *NoopVault) LookupToken(_ context.Context, _ string) (*vapi.Secret, error) {
	return nil, errors.New("Nomad server is not configured to lookup tokens")
}

func (v *NoopVault) RevokeTokens(_ context.Context, tokens []*structs.VaultAccessor, _ bool) error {
	for _, t := range tokens {
		v.logger.Debug("Vault token is no longer used, but Nomad is not able to revoke it. The token may need to be revoked manually or will expire once its TTL reaches zero.", "accessor", t.Accessor, "ttl", t.CreationTTL)
	}

	if err := v.purgeFn(tokens); err != nil {
		v.logger.Error("failed to purge Vault accessors", "error", err)
	}

	return nil
}

func (v *NoopVault) MarkForRevocation(tokens []*structs.VaultAccessor) error {
	for _, t := range tokens {
		v.logger.Debug("Vault token is no longer used, but Nomad is not able to mark it for revocation. The token may need to be revoked manually or will expire once its TTL reaches zero.", "accessor", t.Accessor, "ttl", t.CreationTTL)
	}

	if err := v.purgeFn(tokens); err != nil {
		v.logger.Error("failed to purge Vault accessors", "error", err)
	}

	return nil
}

func (v *NoopVault) Stop() {}

func (v *NoopVault) Running() bool { return true }

func (v *NoopVault) Stats() map[string]string { return nil }

func (v *NoopVault) EmitStats(_ time.Duration, _ <-chan struct{}) {}
