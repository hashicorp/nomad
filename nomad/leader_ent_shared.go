// +build pro ent

package nomad

import (
	"bytes"
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// establishProLeadership is used to establish Nomad Pro systems upon acquiring
// leadership.
func (s *Server) establishProLeadership(stopCh chan struct{}) error {
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
	s.logger.Debug("starting namespace replication from authoritative region", "region", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		// Rate limit how often we attempt replication
		limiter.Wait(context.Background())

		// Fetch the list of namespaces
		var resp structs.NamespaceListResponse
		req.AuthToken = s.ReplicationToken()
		err := s.forwardRegion(s.config.AuthoritativeRegion, "Namespace.ListNamespaces", &req, &resp)
		if err != nil {
			s.logger.Error("failed to fetch namespaces from authoritative region", "error", err)
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
				s.logger.Error("failed to delete namespaces", "error", err)
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
					AuthToken:     s.ReplicationToken(),
					AllowStale:    true,
					MinQueryIndex: resp.Index - 1,
				},
			}
			var reply structs.NamespaceSetResponse
			if err := s.forwardRegion(s.config.AuthoritativeRegion, "Namespace.GetNamespaces", &req, &reply); err != nil {
				s.logger.Error("failed to fetch namespaces from authoritative region", "error", err)
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
				s.logger.Error("failed to update namespaces", "error", err)
				goto ERR_WAIT
			}
		}

		// Update the minimum query index, blocks until there is a change.
		req.MinQueryIndex = resp.Index
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
