// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"crypto/md5"
	"encoding/hex"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

type MockConsulClient struct {
	tokens map[string]*consulapi.ACLToken
}

func NewMockConsulClient(config *config.ConsulConfig, logger hclog.Logger) (Client, error) {
	return &MockConsulClient{}, nil
}

// DeriveTokenWithJWT returns ACLTokens with deterministic values for testing:
// the request ID for the AccessorID and the md5 checksum of the request ID for
// the SecretID
func (mc *MockConsulClient) DeriveTokenWithJWT(req JWTLoginRequest) (*consulapi.ACLToken, error) {
	if t, ok := mc.tokens[req.JWT]; ok {
		return t, nil
	}

	hash := md5.Sum([]byte(req.JWT))
	token := &consulapi.ACLToken{
		AccessorID: hex.EncodeToString(hash[:]),
		SecretID:   hex.EncodeToString(hash[:]),
	}

	if mc.tokens == nil {
		mc.tokens = make(map[string]*consulapi.ACLToken)
	}
	mc.tokens[req.JWT] = token

	return token, nil
}

func (mc *MockConsulClient) RevokeTokens(tokens []*consulapi.ACLToken) error {
	for _, token := range tokens {
		delete(mc.tokens, token.AccessorID)
	}
	return nil
}
