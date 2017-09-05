// +build pro ent

package nomad

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// establishProLeadership is used to establish Nomad Pro systems upon acquiring
// leadership.
func (s *Server) establishProLeadership(stopCh chan struct{}) error {
	if err := s.initializeNamespaces(); err != nil {
		return err
	}

	// Start replication of Namespaces if we are not the authoritative region.
	if s.config.Region != s.config.AuthoritativeRegion {
		go s.replicateNamespaces(stopCh)
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

// replicateNamespaces is used to replicate namespaces from the authoritative
// region to this region.
func (s *Server) replicateNamespaces(stopCh chan struct{}) {
	req := structs.NamespaceListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     s.config.AuthoritativeRegion,
			AllowStale: true,
		},
	}
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Printf("[DEBUG] nomad: starting namespace replication from authoritative region %q", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
			// Rate limit how often we attempt replication
			limiter.Wait(context.Background())

			// Fetch the list of namespaces
			var resp structs.NamespaceListResponse
			req.SecretID = s.ReplicationToken()
			err := s.forwardRegion(s.config.AuthoritativeRegion, "Namespace.ListNamespaces", &req, &resp)
			if err != nil {
				s.logger.Printf("[ERR] nomad: failed to fetch namespaces from authoritative region: %v", err)
				goto ERR_WAIT
			}

			// Perform a two-way diff
			delete, update := diffNamespaces(s.State(), req.MinQueryIndex, resp.Namespaces)

			// Delete namespaces that should not exist
			if len(delete) > 0 {
				args := &structs.NamespaceDeleteRequest{
					Namespaces: delete,
				}
				_, _, err := s.raftApply(structs.NamespaceDeleteRequestType, args)
				if err != nil {
					s.logger.Printf("[ERR] nomad: failed to delete namespaces: %v", err)
					goto ERR_WAIT
				}
			}

			// Fetch any outdated namespaces
			var fetched []*structs.Namespace
			if len(update) > 0 {
				req := structs.NamespaceSetRequest{
					Namespaces: update,
					QueryOptions: structs.QueryOptions{
						Region:        s.config.AuthoritativeRegion,
						SecretID:      s.ReplicationToken(),
						AllowStale:    true,
						MinQueryIndex: resp.Index - 1,
					},
				}
				var reply structs.NamespaceSetResponse
				if err := s.forwardRegion(s.config.AuthoritativeRegion, "Namespace.GetNamespaces", &req, &reply); err != nil {
					s.logger.Printf("[ERR] nomad: failed to fetch namespaces from authoritative region: %v", err)
					goto ERR_WAIT
				}
				for _, namespace := range reply.Namespaces {
					fetched = append(fetched, namespace)
				}
			}

			// Update local namespaces
			if len(fetched) > 0 {
				args := &structs.NamespaceUpsertRequest{
					Namespaces: fetched,
				}
				_, _, err := s.raftApply(structs.NamespaceUpsertRequestType, args)
				if err != nil {
					s.logger.Printf("[ERR] nomad: failed to update namespaces: %v", err)
					goto ERR_WAIT
				}
			}

			// Update the minimum query index, blocks until there is a change.
			req.MinQueryIndex = resp.Index
		}
	}

ERR_WAIT:
	select {
	case <-time.After(s.config.ReplicationBackoff):
		goto START
	case <-stopCh:
		return
	}
}

// diffNamespaces is used to perform a two-way diff between the local namespaces
// and the remote namespaces to determine which namespaces need to be deleted or
// updated.
func diffNamespaces(state *state.StateStore, minIndex uint64, remoteList []*structs.Namespace) (delete []string, update []string) {
	// Construct a set of the local and remote namespaces
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local namespaces
	iter, err := state.Namespaces(nil)
	if err != nil {
		panic("failed to iterate local namespaces")
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		namespace := raw.(*structs.Namespace)
		local[namespace.Name] = namespace.Hash
	}

	// Iterate over the remote namespaces
	for _, rns := range remoteList {
		remote[rns.Name] = struct{}{}

		// Check if the namespace is missing locally
		if localHash, ok := local[rns.Name]; !ok {
			update = append(update, rns.Name)

			// Check if the namespace is newer remotely and there is a hash
			// mis-match.
		} else if rns.ModifyIndex > minIndex && !bytes.Equal(localHash, rns.Hash) {
			update = append(update, rns.Name)
		}
	}

	// Check if namespaces should be deleted
	for lns := range local {
		if _, ok := remote[lns]; !ok {
			delete = append(delete, lns)
		}
	}
	return
}
