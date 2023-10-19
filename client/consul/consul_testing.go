// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

type MockConsulClient struct {
	tokens map[string]string
}

func NewMockConsulClient(config *config.ConsulConfig, logger hclog.Logger) (Client, error) {
	return &MockConsulClient{}, nil
}

// DeriveSITokenWithJWT returns md5 checksums for each of the request IDs
func (mc *MockConsulClient) DeriveSITokenWithJWT(reqs map[string]JWTLoginRequest) (map[string]string, error) {
	if mc.tokens != nil && len(mc.tokens) > 0 {
		return mc.tokens, nil
	}

	tokens := make(map[string]string, len(reqs))
	for id := range reqs {
		hash := md5.Sum([]byte(id))
		tokens[id] = hex.EncodeToString(hash[:])
	}

	return tokens, nil
}
