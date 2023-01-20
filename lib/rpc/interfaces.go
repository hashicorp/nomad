package rpc

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

// Server represents the surface area of a Nomad server necessary for handling
// RPC middleware management.
type Server interface {
	GetLeaderACL() string
	GetNodeID() string
	GetNodeBySecretID() string
	GetRaftConfiguration() (*raft.Configuration, error)
	ResolveSecretToken(string) (*structs.ACLToken, error)
	VerifyClaim(string) (*structs.IdentityClaims, error)
}
