// +build ent

package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Quota endpoint is used for manipulating quotas
type Quota struct {
	srv *Server
}

// UpsertQuotaSpecs is used to upsert a set of quota specifications
func (q *Quota) UpsertQuotaSpecs(args *structs.QuotaSpecUpsertRequest,
	reply *structs.GenericResponse) error {
	if done, err := q.srv.forward("Quota.UpsertQuotaSpecs", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "quota", "upsert_quota_specs"}, time.Now())

	// Check quota write permissions
	if aclObj, err := q.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowQuotaWrite() {
		return structs.ErrPermissionDenied
	}

	// Validate there is at least one quota
	if len(args.Quotas) == 0 {
		return fmt.Errorf("must specify at least one quota")
	}

	// Validate the quota specs and set the hash
	for _, quota := range args.Quotas {
		if err := quota.Validate(); err != nil {
			return fmt.Errorf("Invalid quota %q: %v", quota.Name, err)
		}

		quota.SetHash()
	}

	// Update via Raft
	out, index, err := q.srv.raftApply(structs.QuotaSpecUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Check if there was an error when applying.
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// DeleteQuotaSpecs is used to delete a set of quota specifications
func (q *Quota) DeleteQuotaSpecs(args *structs.QuotaSpecDeleteRequest, reply *structs.GenericResponse) error {
	if done, err := q.srv.forward("Quota.DeleteQuotaSpecs", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "quota", "delete_quota_specs"}, time.Now())

	// Check quota write permissions
	if aclObj, err := q.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowQuotaWrite() {
		return structs.ErrPermissionDenied
	}

	// Validate at least one quota
	if len(args.Names) == 0 {
		return fmt.Errorf("must specify at least one quota specification to delete")
	}

	// Update via Raft
	out, index, err := q.srv.raftApply(structs.QuotaSpecDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Check if there was an error when applying.
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// ListQuotaSpecs is used to list the quota specifications
func (q *Quota) ListQuotaSpecs(args *structs.QuotaSpecListRequest, reply *structs.QuotaSpecListResponse) error {
	if done, err := q.srv.forward("Quota.ListQuotaSpecs", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "quota", "list_quota_specs"}, time.Now())

	// Resolve token to ACL to filter quota specs
	aclObj, err := q.srv.resolveToken(args.SecretID)
	if err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Iterate over all the namespaces
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = s.QuotaSpecsByNamePrefix(ws, prefix)
			} else {
				iter, err = s.QuotaSpecs(ws)
			}
			if err != nil {
				return err
			}

			reply.Quotas = nil
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				qs := raw.(*structs.QuotaSpec)

				// Only return quota allowed by acl
				if allowed, err := q.quotaReadAllowed(qs.Name, s, ws, aclObj); err != nil {
					return err
				} else if allowed {
					reply.Quotas = append(reply.Quotas, qs)
				}
			}

			// Use the last index that affected the namespace table
			index, err := s.Index(state.TableQuotaSpec)
			if err != nil {
				return err
			}

			// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
			// We floor the index at one, since realistically the first write must have a higher index.
			if index == 0 {
				index = 1
			}
			reply.Index = index
			return nil
		}}
	return q.srv.blockingRPC(&opts)
}

// GetQuotaSpec is used to get a specific quota spec
func (q *Quota) GetQuotaSpec(args *structs.QuotaSpecSpecificRequest, reply *structs.SingleQuotaSpecResponse) error {
	if done, err := q.srv.forward("Quota.GetQuotaSpec", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "quota", "get_quota_spec"}, time.Now())

	// Resolve token to ACL
	aclObj, err := q.srv.resolveToken(args.SecretID)
	if err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Only return quota if allowed by acl
			if allowed, err := q.quotaReadAllowed(args.Name, s, ws, aclObj); err != nil {
				return err
			} else if !allowed {
				return structs.ErrPermissionDenied
			}

			// Look for the spec
			out, err := s.QuotaSpecByName(ws, args.Name)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Quota = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the quota table
				index, err := s.Index(state.TableQuotaSpec)
				if err != nil {
					return err
				}

				// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
				// We floor the index at one, since realistically the first write must have a higher index.
				if index == 0 {
					index = 1
				}
				reply.Index = index
			}
			return nil
		}}
	return q.srv.blockingRPC(&opts)
}

// GetQuotaSpecs is used to get a set of quota specs
func (q *Quota) GetQuotaSpecs(args *structs.QuotaSpecSetRequest, reply *structs.QuotaSpecSetResponse) error {
	if done, err := q.srv.forward("Quota.GetQuotaSpecs", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "quota", "get_quota_specs"}, time.Now())

	// Check management level permissions
	if acl, err := q.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if acl != nil && !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Setup the output
			reply.Quotas = make(map[string]*structs.QuotaSpec, len(args.Names))

			// Look for the quota specs
			for _, spec := range args.Names {
				out, err := s.QuotaSpecByName(ws, spec)
				if err != nil {
					return err
				}
				if out != nil {
					reply.Quotas[spec] = out
				}
			}

			// Use the last index that affected the quota table
			index, err := s.Index(state.TableQuotaSpec)
			if err != nil {
				return err
			}

			// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
			// We floor the index at one, since realistically the first write must have a higher index.
			if index == 0 {
				index = 1
			}
			reply.Index = index
			return nil
		}}
	return q.srv.blockingRPC(&opts)
}

// ListQuotaUsages is used to list the quota usages
func (q *Quota) ListQuotaUsages(args *structs.QuotaUsageListRequest, reply *structs.QuotaUsageListResponse) error {
	if done, err := q.srv.forward("Quota.ListQuotaUsages", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "quota", "list_quota_usages"}, time.Now())

	// Resolve token to ACL to filter quota usages
	aclObj, err := q.srv.resolveToken(args.SecretID)
	if err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Iterate over all the namespaces
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = s.QuotaUsagesByNamePrefix(ws, prefix)
			} else {
				iter, err = s.QuotaUsages(ws)
			}
			if err != nil {
				return err
			}

			reply.Usages = nil
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				qu := raw.(*structs.QuotaUsage)

				// Only return quota allowed by acl
				if allowed, err := q.quotaReadAllowed(qu.Name, s, ws, aclObj); err != nil {
					return err
				} else if allowed {
					reply.Usages = append(reply.Usages, qu)
				}
			}

			// Use the last index that affected the namespace table
			index, err := s.Index(state.TableQuotaUsage)
			if err != nil {
				return err
			}

			// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
			// We floor the index at one, since realistically the first write must have a higher index.
			if index == 0 {
				index = 1
			}
			reply.Index = index
			return nil
		}}

	return q.srv.blockingRPC(&opts)
}

// GetQuotaUsage is used to get a specific quota usage
func (q *Quota) GetQuotaUsage(args *structs.QuotaUsageSpecificRequest, reply *structs.SingleQuotaUsageResponse) error {
	if done, err := q.srv.forward("Quota.GetQuotaUsage", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "quota", "get_quota_usage"}, time.Now())

	// Check quota read permissions
	aclObj, err := q.srv.resolveToken(args.SecretID)
	if err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Only return quota allowed by acl
			if allowed, err := q.quotaReadAllowed(args.Name, s, ws, aclObj); err != nil {
				return err
			} else if !allowed {
				return structs.ErrPermissionDenied
			}

			// Look for the usage
			out, err := s.QuotaUsageByName(ws, args.Name)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Usage = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the quota table
				index, err := s.Index(state.TableQuotaUsage)
				if err != nil {
					return err
				}

				// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
				// We floor the index at one, since realistically the first write must have a higher index.
				if index == 0 {
					index = 1
				}
				reply.Index = index
			}
			return nil
		}}
	return q.srv.blockingRPC(&opts)
}

// quotaReadAllowed checks whether the passed ACL token has permission to read
// the given quota object. Its behavior is as follows:
// * If the ACL object is nil, reads are allowed since ACLs are not enabled
// * Reads are allowed if QuotaRead permissions exist on the ACL object
// * Reads are allowed on non-existent quotas to allow blocking queries
// * Reads are allowed if the ACL has any non-deny capability on a namespace
//   using the quota object.
func (q *Quota) quotaReadAllowed(quota string, state *state.StateStore,
	ws memdb.WatchSet, acl *acl.ACL) (bool, error) {

	// ACLs not enabled, so allow the read
	if acl == nil {
		return true, nil
	}

	// Check if they have global read permissions.
	if acl.AllowQuotaRead() {
		return true, nil
	}

	// Check if the quota object exists
	spec, err := state.QuotaSpecByName(ws, quota)
	if err != nil {
		return false, err
	}
	if spec == nil {
		return true, nil
	}

	// Check if the quota spec is attached to a namespace that they have access
	// to.
	iter, err := state.NamespacesByQuota(ws, quota)
	if err != nil {
		return false, err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Check if the ACL gives access to the namespace
		ns := raw.(*structs.Namespace)
		if acl.AllowNamespace(ns.Name) {
			return true, nil
		}
	}

	return false, nil
}
