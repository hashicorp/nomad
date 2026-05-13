// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package vaultclient

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockVaultClient is used for testing the vaultclient integration and is safe
// for concurrent access.
// type MockVaultClient struct {
//
// 	// jwtTokens stores the tokens derived using the JWT flow.
// 	jwtTokens map[string]string
//
// 	// stoppedTokens tracks the tokens that have stopped renewing
// 	stoppedTokens []string
//
// 	// renewTokens are the tokens that have been renewed and their error
// 	// channels
// 	renewTokens map[string]chan error
//
// 	// renewTokenErrors is used to return an error when the RenewToken is called
// 	// with the given token
// 	renewTokenErrors map[string]error
//
// 	// deriveTokenErrors maps an allocation ID and tasks to an error when the
// 	// token is derived
// 	deriveTokenErrors map[string]map[string]error
//
// 	// deriveTokenWithJWTFn allows the caller to control the DeriveTokenWithJWT
// 	// function.
// 	deriveTokenWithJWTFn func(context.Context, JWTLoginRequest) (string, bool, int, error)
//
// 	// renewTokenFn allows the caller to control the Renew function.
// 	renewTokenFn func(context.Context, string, int) (time.Duration, time.Time, error)
//
// 	// renewable determines if the tokens returned should be marked as renewable
// 	renewable bool
//
// 	duration int
//
// 	mu sync.Mutex
// }
//
// // NewMockVaultClient returns a MockVaultClient for testing
// func NewMockVaultClient(_ string) (VaultClient, error) {
// 	return &MockVaultClient{renewable: true, duration: 30}, nil
// }
//
// func (vc *MockVaultClient) DeriveTokenWithJWT(ctx context.Context, req JWTLoginRequest) (string, bool, int, error) {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
//
// 	if vc.deriveTokenWithJWTFn != nil {
// 		return vc.deriveTokenWithJWTFn(ctx, req)
// 	}
//
// 	if vc.jwtTokens == nil {
// 		vc.jwtTokens = make(map[string]string)
// 	}
//
// 	token := uuid.Generate()
// 	if req.Role != "" {
// 		token = fmt.Sprintf("%s-%s", token, req.Role)
// 	}
// 	vc.jwtTokens[req.JWT] = token
// 	return token, vc.renewable, vc.duration, nil
// }
//
// func (vc *MockVaultClient) Renew(ctx context.Context, token string, lease int) (time.Duration, time.Time, error) {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
//
// 	if vc.renewTokenFn != nil {
// 		return vc.renewTokenFn(ctx, token, lease)
// 	}
//
// 	if vc.jwtTokens == nil {
// 		vc.jwtTokens = make(map[string]string)
// 	}
//
// 	vc.jwtTokens[token] = token
// 	return time.Duration(lease), time.Now().Add(time.Duration(lease)), nil
// }
//
// func (vc *MockVaultClient) SetRenewable(renewable bool) {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
// 	vc.renewable = renewable
// }
//
// // JWTTokens returns the tokens generated suing the JWT flow.
// func (vc *MockVaultClient) JWTTokens() map[string]string {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
// 	return vc.jwtTokens
// }
//
// // StoppedTokens tracks the tokens that have stopped renewing
// func (vc *MockVaultClient) StoppedTokens() []string {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
// 	return vc.stoppedTokens
// }
//
// // RenewTokens are the tokens that have been renewed and their error
// // channels
// func (vc *MockVaultClient) RenewTokens() map[string]chan error {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
// 	return vc.renewTokens
// }
//
// // RenewTokenErrCh returns the error channel for the given token renewal
// // process.
// func (vc *MockVaultClient) RenewTokenErrCh(token string) chan error {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
// 	return vc.renewTokens[token]
// }
//
// // SetDeriveTokenWithJWTFn sets the function used to derive tokens using JWT.
// func (vc *MockVaultClient) SetDeriveTokenWithJWTFn(f func(context.Context, JWTLoginRequest) (string, bool, int, error)) {
// 	vc.mu.Lock()
// 	defer vc.mu.Unlock()
// 	vc.deriveTokenWithJWTFn = f
// }

type MockVaultClient struct {
	mock.Mock
}

func NewMockVaultClient() *MockVaultClient {
	return &MockVaultClient{}
}

func (m *MockVaultClient) DeriveTokenWithJWT(ctx context.Context, req JWTLoginRequest) (string, bool, int, error) {
	args := m.Called(ctx, req)
	return args.String(0), args.Bool(1), args.Int(2), args.Error(3)
}

func (m *MockVaultClient) Renew(ctx context.Context, token string, lease int) (time.Duration, error) {
	args := m.Called(ctx, token, lease)
	return args.Get(0).(time.Duration), args.Error(1)
}
