// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

type MockConsulClient struct {
	tokens map[string]string
}

func NewMockConsulClient(config *config.ConsulConfig, logger hclog.Logger) (Client, error) {
	return &MockConsulClient{}, nil
}

func (mc *MockConsulClient) SetTokens(tokens map[string]string) {
	mc.tokens = tokens
}

func (mc *MockConsulClient) DeriveSITokenWithJWT(reqs map[string]JWTLoginRequest) (map[string]string, error) {
	if mc.tokens != nil && len(mc.tokens) > 0 {
		return mc.tokens, nil
	}

	tokens := make(map[string]string, len(reqs))
	for id := range reqs {
		tokens[id] = uuid.Generate()
	}

	return tokens, nil
}
