// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package client

import hclog "github.com/hashicorp/go-hclog"

// EnterpriseClient holds information and methods for enterprise functionality
type EnterpriseClient struct{}

func newEnterpriseClient(logger hclog.Logger) *EnterpriseClient {
	return &EnterpriseClient{}
}

// SetFeatures is used for enterprise builds to configure enterprise features
func (ec *EnterpriseClient) SetFeatures(features uint64) {}
