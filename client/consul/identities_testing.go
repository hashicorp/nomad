// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"sync"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

// MockServiceIdentitiesClient is used for testing the client for managing consul service
// identity tokens.
type MockServiceIdentitiesClient struct {
	// deriveTokenErrors maps an allocation ID and tasks to an error when the
	// token is derived
	deriveTokenErrors map[string]map[string]error

	// DeriveTokenFn allows the caller to control the DeriveToken function. If
	// not set an error is returned if found in DeriveTokenErrors and otherwise
	// a token is generated and returned
	DeriveTokenFn TokenDeriverFunc

	// lock around everything
	lock sync.Mutex
}

var _ ServiceIdentityAPI = (*MockServiceIdentitiesClient)(nil)

// NewMockServiceIdentitiesClient returns a MockServiceIdentitiesClient for testing.
func NewMockServiceIdentitiesClient() *MockServiceIdentitiesClient {
	return &MockServiceIdentitiesClient{
		deriveTokenErrors: make(map[string]map[string]error),
	}
}

func (mtc *MockServiceIdentitiesClient) DeriveSITokens(alloc *structs.Allocation, tasks []string) (map[string]string, error) {
	mtc.lock.Lock()
	defer mtc.lock.Unlock()

	// if the DeriveTokenFn is explicitly set, use that
	if mtc.DeriveTokenFn != nil {
		return mtc.DeriveTokenFn(alloc, tasks)
	}

	// generate a token for each task, unless the mock has an error ready for
	// one or more of the tasks in which case return that
	tokens := make(map[string]string, len(tasks))
	for _, task := range tasks {
		if m, ok := mtc.deriveTokenErrors[alloc.ID]; ok {
			if err, ok := m[task]; ok {
				return nil, err
			}
		}
		tokens[task] = uuid.Generate()
	}
	return tokens, nil
}

func (mtc *MockServiceIdentitiesClient) SetDeriveTokenError(allocID string, tasks []string, err error) {
	mtc.lock.Lock()
	defer mtc.lock.Unlock()

	if _, ok := mtc.deriveTokenErrors[allocID]; !ok {
		mtc.deriveTokenErrors[allocID] = make(map[string]error, 10)
	}

	for _, task := range tasks {
		mtc.deriveTokenErrors[allocID][task] = err
	}
}

func (mtc *MockServiceIdentitiesClient) DeriveTokenErrors() map[string]map[string]error {
	mtc.lock.Lock()
	defer mtc.lock.Unlock()

	m := make(map[string]map[string]error)
	for aID, tasks := range mtc.deriveTokenErrors {
		for task, err := range tasks {
			m[aID][task] = err
		}
	}
	return m
}
