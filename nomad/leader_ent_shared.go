// +build pro ent

package nomad

import (
	"fmt"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/structs"
)

// establishProLeadership is used to establish Nomad Pro systems upon acquiring
// leadership.
func (s *Server) establishProLeadership() error {
	if err := s.initializeNamespaces(); err != nil {
		return err
	}

	return nil
}

// revokeProLeadership is used to disable Nomad Pro systems upon losing
// leadership.
func (s *Server) revokeProLeadership() error {
	return nil
}

// initializeNamespaces is used to setup of Namespaces. If the cluster is just
// being formed or being upgraded to enteprise, the "default" namespace will not
// exist and has to be created.
func (s *Server) initializeNamespaces() error {
	state := s.fsm.State()
	ns, err := state.NamespaceByName(nil, structs.DefaultNamespace)
	if err != nil {
		s.logger.Printf("[ERR] nomad: failed to lookup default namespace when initializing: %v", err)
		return err
	}

	// The default namespace already exists so there is nothing to do.
	if ns != nil {
		return nil
	}

	// Check if the servers are all at a version where they would accept the
	// Raft log.
	// TODO(alex): bump before releasing
	minVersion := version.Must(version.NewVersion("0.7.0-dev"))
	if !ServersMeetMinimumVersion(s.Members(), minVersion) {
		s.logger.Printf("[WARN] nomad: Can't initialize default namespace until all servers are >= %s", minVersion.String())
		return nil
	}

	// Create the default namespace
	req := structs.NamespaceUpsertRequest{
		Namespaces: []*structs.Namespace{
			{
				Name:        structs.DefaultNamespace,
				Description: structs.DefaultNamespaceDescription,
			},
		},
	}
	if _, _, err := s.raftApply(structs.NamespaceUpsertRequestType, &req); err != nil {
		return fmt.Errorf("failed to create default namespace: %v", err)
	}

	return nil
}
