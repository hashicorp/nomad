// +build !ent

package agent

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

func (a *Agent) setupEnterpriseAgent(log hclog.Logger) error {
	// configure eventer
	a.auditor = &noOpAuditor{}

	return nil
}

func (a *Agent) entReloadEventer(cfg *config.AuditConfig) error {
	return nil
}
