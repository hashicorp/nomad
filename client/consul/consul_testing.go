// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"github.com/hashicorp/nomad/helper/uuid"
)

type MockConsulClient struct{}

func (mc *MockConsulClient) DeriveSITokenWithJWT(reqs map[string]JWTLoginRequest) (map[string]string, error) {
	tokens := make(map[string]string, len(reqs))
	for id := range reqs {
		tokens[id] = uuid.Generate()
	}

	return tokens, nil
}
