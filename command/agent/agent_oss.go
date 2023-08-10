// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package agent

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// EnterpriseAgent holds information and methods for enterprise functionality
// in OSS it is an empty struct.
type EnterpriseAgent struct{}

func (a *Agent) setupEnterpriseAgent(log hclog.Logger) error {
	// configure eventer
	a.auditor = &noOpAuditor{}

	return nil
}

func (a *Agent) entReloadEventer(cfg *config.AuditConfig) error {
	return nil
}
