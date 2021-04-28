// +build !ent

package nomad

import (
	"github.com/hashicorp/consul/agent/consul/autopilot"
)

// LicenseConfig allows for tunable licensing config
// primarily used for enterprise testing
type LicenseConfig struct {
	AdditionalPubKeys []string
}

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
	s.autopilot = autopilot.NewAutopilot(s.logger, apDelegate, config.AutopilotInterval, config.ServerHealthInterval)

	return nil
}
func (s *Server) startEnterpriseBackground() {}

func (s *Server) entVaultDelegate() *VaultNoopDelegate {
	return &VaultNoopDelegate{}
}
