// +build pro ent

package nomad

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/consul/autopilot_ent"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

// AdvancedAutopilotDelegate defines a policy for promoting non-voting servers in a way
// that maintains an odd-numbered voter count while respecting configured redundancy
// zones, servers marked non-voter, and any upgrade migrations to perform.
type AdvancedAutopilotDelegate struct {
	AutopilotDelegate

	promoter *autopilot_ent.AdvancedPromoter
}

func (d *AdvancedAutopilotDelegate) PromoteNonVoters(conf *autopilot.Config, health autopilot.OperatorHealthReply) ([]raft.Server, error) {
	minRaftProtocol, err := d.server.autopilot.MinRaftProtocol()
	if err != nil {
		return nil, fmt.Errorf("error getting server raft protocol versions: %s", err)
	}

	// If we don't meet the minimum version for non-voter features, bail early
	if minRaftProtocol < 3 {
		return nil, nil
	}

	return d.promoter.PromoteNonVoters(conf, health)
}

// getNodeMeta tries to fetch a node's metadata
func (s *Server) getNodeMeta(serverID raft.ServerID) (map[string]string, error) {
	meta := make(map[string]string)
	for _, member := range s.Members() {
		ok, parts := isNomadServer(member)
		if !ok || raft.ServerID(parts.ID) != serverID {
			continue
		}

		meta[AutopilotRZTag] = member.Tags[AutopilotRZTag]
		meta[AutopilotVersionTag] = member.Tags[AutopilotVersionTag]
		break
	}

	return meta, nil
}

// Set up the enterprise version of autopilot
func (s *Server) setupEnterpriseAutopilot(config *Config) {
	apDelegate := &AdvancedAutopilotDelegate{
		AutopilotDelegate: AutopilotDelegate{server: s},
	}
	stdLogger := s.logger.StandardLogger(&log.StandardLoggerOptions{InferLevels: true})
	apDelegate.promoter = autopilot_ent.NewAdvancedPromoter(stdLogger, apDelegate, s.getNodeMeta)
	s.autopilot = autopilot.NewAutopilot(stdLogger, apDelegate, config.AutopilotInterval, config.ServerHealthInterval)
}
