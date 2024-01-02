// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

// TestVaultClient is a Vault client appropriate for use during testing. Its
// behavior is programmable such that endpoints can be tested under various
// circumstances.
type TestVaultClient struct {
	// LookupTokenErrors maps a token to an error that will be returned by the
	// LookupToken call
	LookupTokenErrors map[string]error

	// LookupTokenSecret maps a token to the Vault secret that will be returned
	// by the LookupToken call
	LookupTokenSecret map[string]*vapi.Secret

	// CreateTokenErrors maps a token to an error that will be returned by the
	// CreateToken call
	CreateTokenErrors map[string]map[string]error

	// CreateTokenSecret maps a token to the Vault secret that will be returned
	// by the CreateToken call
	CreateTokenSecret map[string]map[string]*vapi.Secret

	RevokedTokens []*structs.VaultAccessor
}

func (v *TestVaultClient) LookupToken(ctx context.Context, token string) (*vapi.Secret, error) {
	var secret *vapi.Secret
	var err error

	if v.LookupTokenSecret != nil {
		secret = v.LookupTokenSecret[token]
	}
	if v.LookupTokenErrors != nil {
		err = v.LookupTokenErrors[token]
	}

	return secret, err
}

// SetLookupTokenError sets the error that will be returned by the token
// lookup
func (v *TestVaultClient) SetLookupTokenError(token string, err error) {
	if v.LookupTokenErrors == nil {
		v.LookupTokenErrors = make(map[string]error)
	}

	v.LookupTokenErrors[token] = err
}

// SetLookupTokenSecret sets the secret that will be returned by the token
// lookup
func (v *TestVaultClient) SetLookupTokenSecret(token string, secret *vapi.Secret) {
	if v.LookupTokenSecret == nil {
		v.LookupTokenSecret = make(map[string]*vapi.Secret)
	}

	v.LookupTokenSecret[token] = secret
}

// SetLookupTokenAllowedPolicies is a helper that adds a secret that allows the
// given policies
func (v *TestVaultClient) SetLookupTokenAllowedPolicies(token string, policies []string) {
	s := &vapi.Secret{
		Data: map[string]interface{}{
			"policies": policies,
		},
	}

	v.SetLookupTokenSecret(token, s)
}

func (v *TestVaultClient) CreateToken(ctx context.Context, a *structs.Allocation, task string) (*vapi.Secret, error) {
	var secret *vapi.Secret
	var err error

	if v.CreateTokenSecret != nil {
		tasks := v.CreateTokenSecret[a.ID]
		if tasks != nil {
			secret = tasks[task]
		}
	}
	if v.CreateTokenErrors != nil {
		tasks := v.CreateTokenErrors[a.ID]
		if tasks != nil {
			err = tasks[task]
		}
	}

	return secret, err
}

// SetCreateTokenError sets the error that will be returned by the token
// creation
func (v *TestVaultClient) SetCreateTokenError(allocID, task string, err error) {
	if v.CreateTokenErrors == nil {
		v.CreateTokenErrors = make(map[string]map[string]error)
	}

	tasks := v.CreateTokenErrors[allocID]
	if tasks == nil {
		tasks = make(map[string]error)
		v.CreateTokenErrors[allocID] = tasks
	}

	v.CreateTokenErrors[allocID][task] = err
}

// SetCreateTokenSecret sets the secret that will be returned by the token
// creation
func (v *TestVaultClient) SetCreateTokenSecret(allocID, task string, secret *vapi.Secret) {
	if v.CreateTokenSecret == nil {
		v.CreateTokenSecret = make(map[string]map[string]*vapi.Secret)
	}

	tasks := v.CreateTokenSecret[allocID]
	if tasks == nil {
		tasks = make(map[string]*vapi.Secret)
		v.CreateTokenSecret[allocID] = tasks
	}

	v.CreateTokenSecret[allocID][task] = secret
}

func (v *TestVaultClient) RevokeTokens(ctx context.Context, accessors []*structs.VaultAccessor, committed bool) error {
	v.RevokedTokens = append(v.RevokedTokens, accessors...)
	return nil
}

func (v *TestVaultClient) MarkForRevocation(accessors []*structs.VaultAccessor) error {
	v.RevokedTokens = append(v.RevokedTokens, accessors...)
	return nil
}

func (v *TestVaultClient) Stop()                                                  {}
func (v *TestVaultClient) SetActive(enabled bool)                                 {}
func (v *TestVaultClient) GetConfig() *config.VaultConfig                         { return nil }
func (v *TestVaultClient) SetConfig(config *config.VaultConfig) error             { return nil }
func (v *TestVaultClient) Running() bool                                          { return true }
func (v *TestVaultClient) Stats() map[string]string                               { return map[string]string{} }
func (v *TestVaultClient) EmitStats(period time.Duration, stopCh <-chan struct{}) {}
