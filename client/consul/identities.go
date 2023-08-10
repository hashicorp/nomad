// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Implementation of ServiceIdentityAPI used to interact with Nomad Server from
// Nomad Client for acquiring Consul Service Identity tokens.
//
// This client is split from the other consul client(s) to avoid a circular
// dependency between themselves and client.Client
type identitiesClient struct {
	tokenDeriver TokenDeriverFunc
	logger       hclog.Logger
}

func NewIdentitiesClient(logger hclog.Logger, tokenDeriver TokenDeriverFunc) *identitiesClient {
	return &identitiesClient{
		tokenDeriver: tokenDeriver,
		logger:       logger,
	}
}

func (c *identitiesClient) DeriveSITokens(alloc *structs.Allocation, tasks []string) (map[string]string, error) {
	tokens, err := c.tokenDeriver(alloc, tasks)
	if err != nil {
		c.logger.Error("error deriving SI token", "error", err, "alloc_id", alloc.ID, "task_names", tasks)
		return nil, err
	}
	return tokens, nil
}
