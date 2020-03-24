// +build !ent

package agent

import (
	"context"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/command/agent/event"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

type noOpAuditor struct{}

// Ensure noOpAuditor is an Eventer
var _ event.Auditor = &noOpAuditor{}

func (e *noOpAuditor) Event(ctx context.Context, eventType string, payload interface{}) error {
	return nil
}

func (e *noOpAuditor) Enabled() bool {
	return false
}

func (e *noOpAuditor) Reopen() error {
	return nil
}

func (e *noOpAuditor) SetEnabled(enabled bool) {}

func (e *noOpAuditor) DeliveryEnforced() bool { return false }

func (a *Agent) setupEnterpriseAgent(log hclog.Logger) error {
	// configure eventer
	a.auditor = &noOpAuditor{}

	return nil
}

func (a *Agent) entReloadEventer(cfg *config.AuditConfig) error {
	return nil
}
