// +build !pro,!ent

package nomad

import (
	"github.com/hashicorp/consul/agent/consul/autopilot"
	log "github.com/hashicorp/go-hclog"
)

type EnterpriseState struct{}

func (s *Server) setupEnterprise(config *Config) error {
	// Set up the OSS version of autopilot
	apDelegate := &AutopilotDelegate{s}
	s.autopilot = autopilot.NewAutopilot(s.logger.StandardLogger(&log.StandardLoggerOptions{InferLevels: true}), apDelegate, config.AutopilotInterval, config.ServerHealthInterval)

	return nil
}

func (s *Server) startEnterpriseBackground() {}
