// +build !pro,!ent

package nomad

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/autopilot"
)

// LicenseConfig allows for tunable licensing config
// primarily used for enterprise testing
type LicenseConfig struct{}

type EnterpriseState struct{}

func (es *EnterpriseState) FeatureCheckPreemption() error {
	return fmt.Errorf("Feature \"Preemption\" is unlicensed")
}

func (es *EnterpriseState) Features() uint64 {
	return 0
}

func (s *Server) setupEnterprise(config *Config) error {
	// Set up the OSS version of autopilot
	apDelegate := &AutopilotDelegate{s}
	s.autopilot = autopilot.NewAutopilot(s.logger, apDelegate, config.AutopilotInterval, config.ServerHealthInterval)

	return nil
}

func (s *Server) startEnterpriseBackground() {}
