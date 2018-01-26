// +build ent

package nomad

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/consul/autopilot_ent"
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
func (s *Server) getNodeMeta(id raft.ServerID) (map[string]string, error) {
	state := s.fsm.State()
	node, err := state.NodeByID(nil, string(id))
	if err != nil {
		return nil, fmt.Errorf("error retrieving node from state store: %s", err)
	}
	if node == nil {
		return nil, fmt.Errorf("no catalog entry for server ID %q", id)
	}

	return node.Meta, nil
}
