// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

type NoopVault struct {
	l      sync.Mutex
	config *config.VaultConfig
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
	return nil, errors.New("Vault client not able to create tokens")
}

func (v *NoopVault) LookupToken(_ context.Context, _ string) (*vapi.Secret, error) {
	return nil, errors.New("Vault client not able to lookup tokens")
}

func (v *NoopVault) RevokeTokens(_ context.Context, _ []*structs.VaultAccessor, _ bool) error {
	return errors.New("Vault client not able to revoke tokens")
}

func (v *NoopVault) MarkForRevocation(accessors []*structs.VaultAccessor) error {
	return errors.New("Vault client not able to revoke tokens")
}

func (v *NoopVault) Stop() {}

func (v *NoopVault) Running() bool { return true }

func (v *NoopVault) Stats() map[string]string { return nil }

func (v *NoopVault) EmitStats(_ time.Duration, _ <-chan struct{}) {}
