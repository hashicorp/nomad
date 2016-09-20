package vaultclient

import (
	"github.com/hashicorp/nomad/nomad/structs"
	vaultapi "github.com/hashicorp/vault/api"
)

// MockVaultClient is used for testing the vaultclient integration
type MockVaultClient struct {
	// StoppedTokens tracks the tokens that have stopped renewing
	StoppedTokens []string

	// RenewTokens are the tokens that have been renewed and their error
	// channels
	RenewTokens map[string]chan error

	// RenewTokenErrors is used to return an error when the RenewToken is called
	// with the given token
	RenewTokenErrors map[string]error

	// DeriveTokenErrors maps an allocation ID and tasks to an error when the
	// token is derived
	DeriveTokenErrors map[string]map[string]error
}

// NewMockVaultClient returns a MockVaultClient for testing
func NewMockVaultClient() *MockVaultClient { return &MockVaultClient{} }

func (vc *MockVaultClient) DeriveToken(a *structs.Allocation, tasks []string) (map[string]string, error) {
	tokens := make(map[string]string, len(tasks))
	for _, task := range tasks {
		if tasks, ok := vc.DeriveTokenErrors[a.ID]; ok {
			if err, ok := tasks[task]; ok {
				return nil, err
			}
		}

		tokens[task] = structs.GenerateUUID()
	}

	return tokens, nil
}

func (vc *MockVaultClient) SetDeriveTokenError(allocID string, tasks []string, err error) {
	if vc.DeriveTokenErrors == nil {
		vc.DeriveTokenErrors = make(map[string]map[string]error, 10)
	}

	if _, ok := vc.RenewTokenErrors[allocID]; !ok {
		vc.DeriveTokenErrors[allocID] = make(map[string]error, 10)
	}

	for _, task := range tasks {
		vc.DeriveTokenErrors[allocID][task] = err
	}
}

func (vc *MockVaultClient) RenewToken(token string, interval int) (<-chan error, error) {
	if err, ok := vc.RenewTokenErrors[token]; ok {
		return nil, err
	}

	renewCh := make(chan error)
	if vc.RenewTokens == nil {
		vc.RenewTokens = make(map[string]chan error, 10)
	}
	vc.RenewTokens[token] = renewCh
	return renewCh, nil
}

func (vc *MockVaultClient) SetRenewTokenError(token string, err error) {
	if vc.RenewTokenErrors == nil {
		vc.RenewTokenErrors = make(map[string]error, 10)
	}

	vc.RenewTokenErrors[token] = err
}

func (vc *MockVaultClient) StopRenewToken(token string) error {
	vc.StoppedTokens = append(vc.StoppedTokens, token)
	return nil
}

func (vc *MockVaultClient) RenewLease(leaseId string, interval int) (<-chan error, error) {
	return nil, nil
}
func (vc *MockVaultClient) StopRenewLease(leaseId string) error                   { return nil }
func (vc *MockVaultClient) Start()                                                {}
func (vc *MockVaultClient) Stop()                                                 {}
func (vc *MockVaultClient) GetConsulACL(string, string) (*vaultapi.Secret, error) { return nil, nil }
