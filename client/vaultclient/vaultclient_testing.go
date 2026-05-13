// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package vaultclient

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

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
