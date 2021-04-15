package structs

import (
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/serf/serf"
)

// AgentSelfRequest is used by /agent/self to retrieve the agent's
// configuration and statistics. If ServerID or NodeID is specified, the
// request is forwarded to the remote agent
type AgentSelfRequest struct {
	ServerID string
	NodeID   string
	QueryOptions
}

// // AgentSelfResponse contains the HostData content
// // Stats can be either server.Stats or client.Stats
// //   the other option is to formally declarea struct for each
// //   but that that may be more static than desired
// // We also could return both client and server when they are both running
type AgentSelfResponse struct {
	AgentID string
	Config  *agent.Config
	Member  serf.Member
	Stats   map[string]map[string]string
}
