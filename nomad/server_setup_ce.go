// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package nomad

import (
	autopilot "github.com/hashicorp/raft-autopilot"
)

type EnterpriseState struct{}

func (es *EnterpriseState) Features() uint64 {
	return 0
}

func (es *EnterpriseState) ReloadLicense(_ *Config) error {
	return nil
}

func (s *Server) setupEnterprise(config *Config) error {
	// Set up the OSS version of autopilot
	apDelegate := &AutopilotDelegate{s}

	s.autopilot = autopilot.New(
		s.raft,
		apDelegate,
		autopilot.WithLogger(s.logger),
		autopilot.WithReconcileInterval(config.AutopilotInterval),
		autopilot.WithUpdateInterval(config.ServerHealthInterval),
		autopilot.WithPromoter(s.autopilotPromoter()),
	)

	return nil
}
func (s *Server) startEnterpriseBackground() {}

func (s *Server) entVaultDelegate() *VaultNoopDelegate {
	return &VaultNoopDelegate{}
}
