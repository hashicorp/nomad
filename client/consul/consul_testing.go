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
func (mc *MockConsulClient) DeriveTokenWithJWT(reqs map[string]JWTLoginRequest) (map[string]*consulapi.ACLToken, error) {
	if mc.tokens != nil && len(mc.tokens) > 0 {
		return mc.tokens, nil
	}

	tokens := make(map[string]*consulapi.ACLToken, len(reqs))
	for id := range reqs {
		hash := md5.Sum([]byte(id))
		token := &consulapi.ACLToken{
			AccessorID: id,
			SecretID:   hex.EncodeToString(hash[:]),
		}
		tokens[id] = token
	}

	return tokens, nil
}

func (mc *MockConsulClient) RevokeTokens(tokens []*consulapi.ACLToken) error {
	for _, token := range tokens {
		delete(mc.tokens, token.AccessorID)
	}
	return nil
}
