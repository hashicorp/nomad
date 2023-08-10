// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultclient

import (
	"sync"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	vaultapi "github.com/hashicorp/vault/api"
)

// MockVaultClient is used for testing the vaultclient integration and is safe
// for concurrent access.
type MockVaultClient struct {
	// stoppedTokens tracks the tokens that have stopped renewing
	stoppedTokens []string

	// renewTokens are the tokens that have been renewed and their error
	// channels
	renewTokens map[string]chan error

	// renewTokenErrors is used to return an error when the RenewToken is called
	// with the given token
	renewTokenErrors map[string]error

	// deriveTokenErrors maps an allocation ID and tasks to an error when the
	// token is derived
	deriveTokenErrors map[string]map[string]error

	// DeriveTokenFn allows the caller to control the DeriveToken function. If
	// not set an error is returned if found in DeriveTokenErrors and otherwise
	// a token is generated and returned
	DeriveTokenFn func(a *structs.Allocation, tasks []string) (map[string]string, error)

	mu sync.Mutex
}

// NewMockVaultClient returns a MockVaultClient for testing
func NewMockVaultClient() *MockVaultClient { return &MockVaultClient{} }

func (vc *MockVaultClient) DeriveToken(a *structs.Allocation, tasks []string) (map[string]string, error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if vc.DeriveTokenFn != nil {
		return vc.DeriveTokenFn(a, tasks)
	}

	tokens := make(map[string]string, len(tasks))
	for _, task := range tasks {
		if tasks, ok := vc.deriveTokenErrors[a.ID]; ok {
			if err, ok := tasks[task]; ok {
				return nil, err
			}
		}

		tokens[task] = uuid.Generate()
	}

	return tokens, nil
}

func (vc *MockVaultClient) SetDeriveTokenError(allocID string, tasks []string, err error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if vc.deriveTokenErrors == nil {
		vc.deriveTokenErrors = make(map[string]map[string]error, 10)
	}

	if _, ok := vc.deriveTokenErrors[allocID]; !ok {
		vc.deriveTokenErrors[allocID] = make(map[string]error, 10)
	}

	for _, task := range tasks {
		vc.deriveTokenErrors[allocID][task] = err
	}
}

func (vc *MockVaultClient) RenewToken(token string, interval int) (<-chan error, error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if err, ok := vc.renewTokenErrors[token]; ok {
		return nil, err
	}

	renewCh := make(chan error)
	if vc.renewTokens == nil {
		vc.renewTokens = make(map[string]chan error, 10)
	}
	vc.renewTokens[token] = renewCh
	return renewCh, nil
}

func (vc *MockVaultClient) SetRenewTokenError(token string, err error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if vc.renewTokenErrors == nil {
		vc.renewTokenErrors = make(map[string]error, 10)
	}

	vc.renewTokenErrors[token] = err
}

func (vc *MockVaultClient) StopRenewToken(token string) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.stoppedTokens = append(vc.stoppedTokens, token)
	return nil
}

func (vc *MockVaultClient) Start() {}

func (vc *MockVaultClient) Stop() {}

func (vc *MockVaultClient) GetConsulACL(string, string) (*vaultapi.Secret, error) { return nil, nil }

// StoppedTokens tracks the tokens that have stopped renewing
func (vc *MockVaultClient) StoppedTokens() []string {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return vc.stoppedTokens
}

// RenewTokens are the tokens that have been renewed and their error
// channels
func (vc *MockVaultClient) RenewTokens() map[string]chan error {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return vc.renewTokens
}

// RenewTokenErrors is used to return an error when the RenewToken is called
// with the given token
func (vc *MockVaultClient) RenewTokenErrors() map[string]error {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return vc.renewTokenErrors
}

// DeriveTokenErrors maps an allocation ID and tasks to an error when the
// token is derived
func (vc *MockVaultClient) DeriveTokenErrors() map[string]map[string]error {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return vc.deriveTokenErrors
}
