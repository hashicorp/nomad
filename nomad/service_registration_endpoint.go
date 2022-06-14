package nomad

import (
	"net/http"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ServiceRegistration encapsulates the service registrations RPC endpoint
// which is callable via the ServiceRegistration RPCs and externally via the
// "/v1/service{s}" HTTP API.
type ServiceRegistration struct {
	srv *Server

	// ctx provides context regarding the underlying connection, so we can
	// perform TLS certificate validation on internal only endpoints.
	ctx *RPCContext
}

// Upsert creates or updates service registrations held within Nomad. This RPC
// is only callable by Nomad nodes.
func (s *ServiceRegistration) Upsert(
	args *structs.ServiceRegistrationUpsertRequest,
	reply *structs.ServiceRegistrationUpsertResponse) error {

	// Ensure the connection was initiated by a client if TLS is used.
	if err := validateTLSCertificateLevel(s.srv, s.ctx, tlsCertificateLevelClient); err != nil {
		return err
	}

	if done, err := s.srv.forward(structs.ServiceRegistrationUpsertRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "upsert"}, time.Now())

	// This endpoint is only callable by nodes in the cluster. Therefore,
	// perform a node lookup using the secret ID to confirm the caller is a
	// known node.
	node, err := s.srv.fsm.State().NodeBySecretID(nil, args.AuthToken)
	if err != nil {
		return err
	}
	if node == nil {
		return structs.ErrTokenNotFound
	}

	// Use a multierror, so we can capture all validation errors and pass this
	// back so fixing in a single swoop.
	var mErr multierror.Error

	// Iterate the services and validate them. Any error results in the call
	// failing.
	for _, service := range args.Services {
		if err := service.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	if err := mErr.ErrorOrNil(); err != nil {
		return err
	}

	// Update via Raft.
	out, index, err := s.srv.raftApply(structs.ServiceRegistrationUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Check if the FSM response, which is an interface, contains an error.
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// Update the index. There is no need to floor this as we are writing to
	// state and therefore will get a non-zero index response.
	reply.Index = index
	return nil
}

// DeleteByID removes a single service registration, as specified by its ID
// from Nomad. This is typically called by Nomad nodes, however, in extreme
// situations can be used via the CLI and API by operators.
func (s *ServiceRegistration) DeleteByID(
	args *structs.ServiceRegistrationDeleteByIDRequest,
	reply *structs.ServiceRegistrationDeleteByIDResponse) error {

	if done, err := s.srv.forward(structs.ServiceRegistrationDeleteByIDRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "delete_id"}, time.Now())

	// Perform the ACL token resolution.
	aclObj, err := s.srv.ResolveToken(args.AuthToken)

	switch err {
	case nil:
		// If ACLs are enabled, ensure the caller has the submit-job namespace
		// capability.
		if aclObj != nil {
			hasSubmitJob := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob)
			if !hasSubmitJob {
				return structs.ErrPermissionDenied
			}
		}
	default:
		// This endpoint is generally called by Nomad nodes, so we want to
		// perform this check, unless the token resolution gave us a terminal
		// error.
		if err != structs.ErrTokenNotFound {
			return err
		}

		// Attempt to lookup AuthToken as a Node.SecretID and return any error
		// wrapped along with the original.
		node, stateErr := s.srv.fsm.State().NodeBySecretID(nil, args.AuthToken)
		if stateErr != nil {
			var mErr multierror.Error
			mErr.Errors = append(mErr.Errors, err, stateErr)
			return mErr.ErrorOrNil()
		}

		// At this point, we do not have a valid ACL token, nor are we being
		// called, or able to confirm via the state store, by a node.
		if node == nil {
			return structs.ErrTokenNotFound
		}
	}

	// Update via Raft.
	out, index, err := s.srv.raftApply(structs.ServiceRegistrationDeleteByIDRequestType, args)
	if err != nil {
		return err
	}

	// Check if the FSM response, which is an interface, contains an error.
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// Update the index. There is no need to floor this as we are writing to
	// state and therefore will get a non-zero index response.
	reply.Index = index
	return nil
}

// List is used to list service registration held within state. It supports
// single and wildcard namespace listings.
func (s *ServiceRegistration) List(
	args *structs.ServiceRegistrationListRequest,
	reply *structs.ServiceRegistrationListResponse) error {

	if done, err := s.srv.forward(structs.ServiceRegistrationListRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "list"}, time.Now())

	// If the caller has requested to list services across all namespaces, use
	// the custom function to perform this.
	if args.RequestNamespace() == structs.AllNamespacesSentinel {
		return s.listAllServiceRegistrations(args, reply)
	}

	// Perform our mixed auth handling.
	if err := s.handleMixedAuthEndpoint(args.QueryOptions, acl.NamespaceCapabilityReadJob); err != nil {
		return err
	}

	// Set up and return the blocking query.
	return s.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Perform the state query to get an iterator.
			iter, err := stateStore.GetServiceRegistrationsByNamespace(ws, args.RequestNamespace())
			if err != nil {
				return err
			}

			// Track the unique tags found per service registration name.
			serviceTags := make(map[string]map[string]struct{})

			for raw := iter.Next(); raw != nil; raw = iter.Next() {

				serviceReg := raw.(*structs.ServiceRegistration)

				// Identify and add any tags for the current service being
				// iterated into the map. If the tag has already been seen for
				// the same service, it will be overwritten ensuring no
				// duplicates.
				tags, ok := serviceTags[serviceReg.ServiceName]
				if !ok {
					serviceTags[serviceReg.ServiceName] = make(map[string]struct{})
					tags = serviceTags[serviceReg.ServiceName]
				}
				for _, tag := range serviceReg.Tags {
					tags[tag] = struct{}{}
				}
			}

			var serviceList []*structs.ServiceRegistrationStub

			// Iterate the serviceTags map and populate our output result. This
			// endpoint handles a single namespace, so we do not need to
			// account for multiple.
			for service, tags := range serviceTags {

				serviceStub := structs.ServiceRegistrationStub{
					ServiceName: service,
					Tags:        make([]string, 0, len(tags)),
				}
				for tag := range tags {
					serviceStub.Tags = append(serviceStub.Tags, tag)
				}

				serviceList = append(serviceList, &serviceStub)
			}

			// Correctly handle situations where a namespace was passed that
			// either does not contain service registrations, or might not even
			// exist.
			if len(serviceList) > 0 {
				reply.Services = []*structs.ServiceRegistrationListStub{
					{
						Namespace: args.RequestNamespace(),
						Services:  serviceList,
					},
				}
			} else {
				reply.Services = make([]*structs.ServiceRegistrationListStub, 0)
			}

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return s.srv.setReplyQueryMeta(stateStore, state.TableServiceRegistrations, &reply.QueryMeta)
		},
	})
}

// listAllServiceRegistrations is used to list service registration held within
// state where the caller has used the namespace wildcard identifier.
func (s *ServiceRegistration) listAllServiceRegistrations(
	args *structs.ServiceRegistrationListRequest,
	reply *structs.ServiceRegistrationListResponse) error {

	// Perform token resolution. The request already goes through forwarding
	// and metrics setup before being called.
	aclObj, err := s.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}

	// allowFunc checks whether the caller has the read-job capability on the
	// passed namespace.
	allowFunc := func(ns string) bool {
		return aclObj.AllowNsOp(ns, acl.NamespaceCapabilityReadJob)
	}

	// Set up and return the blocking query.
	return s.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Identify which namespaces the caller has access to. If they do
			// not have access to any, send them an empty response. Otherwise,
			// handle any error in a traditional manner.
			allowedNSes, err := allowedNSes(aclObj, stateStore, allowFunc)
			switch err {
			case structs.ErrPermissionDenied:
				reply.Services = make([]*structs.ServiceRegistrationListStub, 0)
				return nil
			case nil:
				// Fallthrough.
			default:
				return err
			}

			// Get all the service registrations stored within state.
			iter, err := stateStore.GetServiceRegistrations(ws)
			if err != nil {
				return err
			}

			// Track the unique tags found per namespace per service
			// registration name.
			namespacedServiceTags := make(map[string]map[string]map[string]struct{})

			// Iterate all service registrations.
			for raw := iter.Next(); raw != nil; raw = iter.Next() {

				// We need to assert the type here in order to check the
				// namespace.
				serviceReg := raw.(*structs.ServiceRegistration)

				// Check whether the service registration is within a namespace
				// the caller is permitted to view. nil allowedNSes means the
				// caller can view all namespaces.
				if allowedNSes != nil && !allowedNSes[serviceReg.Namespace] {
					continue
				}

				// Identify and add any tags for the current namespaced service
				// being iterated into the map. If the tag has already been
				// seen for the same service, it will be overwritten ensuring
				// no duplicates.
				namespace, ok := namespacedServiceTags[serviceReg.Namespace]
				if !ok {
					namespacedServiceTags[serviceReg.Namespace] = make(map[string]map[string]struct{})
					namespace = namespacedServiceTags[serviceReg.Namespace]
				}
				tags, ok := namespace[serviceReg.ServiceName]
				if !ok {
					namespace[serviceReg.ServiceName] = make(map[string]struct{})
					tags = namespace[serviceReg.ServiceName]
				}
				for _, tag := range serviceReg.Tags {
					tags[tag] = struct{}{}
				}
			}

			// Set up our output object. Start with zero size but allocate the
			// know length as we wil need to append whilst avoid slice growing.
			servicesOutput := make([]*structs.ServiceRegistrationListStub, 0, len(namespacedServiceTags))

			for ns, serviceTags := range namespacedServiceTags {

				var serviceList []*structs.ServiceRegistrationStub

				// Iterate the serviceTags map and populate our output result.
				for service, tags := range serviceTags {

					serviceStub := structs.ServiceRegistrationStub{
						ServiceName: service,
						Tags:        make([]string, 0, len(tags)),
					}
					for tag := range tags {
						serviceStub.Tags = append(serviceStub.Tags, tag)
					}

					serviceList = append(serviceList, &serviceStub)
				}

				servicesOutput = append(servicesOutput, &structs.ServiceRegistrationListStub{
					Namespace: ns,
					Services:  serviceList,
				})
			}

			// Add the output to the reply object.
			reply.Services = servicesOutput

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return s.srv.setReplyQueryMeta(stateStore, state.TableServiceRegistrations, &reply.QueryMeta)
		},
	})
}

// GetService is used to get all services registrations corresponding to a
// single name.
func (s *ServiceRegistration) GetService(
	args *structs.ServiceRegistrationByNameRequest,
	reply *structs.ServiceRegistrationByNameResponse) error {

	if done, err := s.srv.forward(structs.ServiceRegistrationGetServiceRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "get_service"}, time.Now())

	// Perform our mixed auth handling.
	if err := s.handleMixedAuthEndpoint(args.QueryOptions, acl.NamespaceCapabilityReadJob); err != nil {
		return err
	}

	// Set up the blocking query.
	return s.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Perform the state query to get an iterator.
			iter, err := stateStore.GetServiceRegistrationByName(ws, args.RequestNamespace(), args.ServiceName)
			if err != nil {
				return err
			}

			// Generate the tokenizer to use for pagination using namespace and
			// ID to ensure complete uniqueness.
			tokenizer := paginator.NewStructsTokenizer(iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				},
			)

			// Set up our output after we have checked the error.
			var services []*structs.ServiceRegistration

			// Build the paginator. This includes the function that is
			// responsible for appending a registration to the services array.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					services = append(services, raw.(*structs.ServiceRegistration))
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			// Calling page populates our output services array as well as
			// returns the next token.
			nextToken, err := paginatorImpl.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			// Populate the reply.
			reply.Services = services
			reply.NextToken = nextToken

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return s.srv.setReplyQueryMeta(stateStore, state.TableServiceRegistrations, &reply.QueryMeta)
		},
	})
}

// handleMixedAuthEndpoint is a helper to handle auth on RPC endpoints that can
// either be called by Nomad nodes, or by external clients.
func (s *ServiceRegistration) handleMixedAuthEndpoint(args structs.QueryOptions, cap string) error {

	// Perform the initial token resolution.
	aclObj, err := s.srv.ResolveToken(args.AuthToken)

	switch err {
	case nil:
		// Perform our ACL validation. If the object is nil, this means ACLs
		// are not enabled, otherwise trigger the allowed namespace function.
		if aclObj != nil {
			if !aclObj.AllowNsOp(args.RequestNamespace(), cap) {
				return structs.ErrPermissionDenied
			}
		}
	default:
		// Attempt to verify the token as a JWT with a workload
		// identity claim if it's not a secret ID.
		// COMPAT(1.4.0): we can remove this conditional in 1.5.0
		if !helper.IsUUID(args.AuthToken) {
			claims, err := s.srv.VerifyClaim(args.AuthToken)
			if err != nil {
				return err
			}
			if claims == nil {
				return structs.ErrPermissionDenied
			}
			return nil
		}

		// COMPAT(1.4.0): Nomad 1.3.0 shipped with authentication by
		// node secret but that's been replaced with workload identity
		// in 1.4.0. Leave this here for backwards compatibility
		// between clients and servers during cluster upgrades, but
		// remove for 1.5.0

		// In the event we got any error other than notfound, consider this
		// terminal.
		if err != structs.ErrTokenNotFound {
			return err
		}

		// Attempt to lookup AuthToken as a Node.SecretID and
		// return any error wrapped along with the original.
		node, stateErr := s.srv.fsm.State().NodeBySecretID(nil, args.AuthToken)
		if stateErr != nil {
			var mErr multierror.Error
			mErr.Errors = append(mErr.Errors, err, stateErr)
			return mErr.ErrorOrNil()
		}
		// At this point, we do not have a valid ACL token, nor are we being
		// called, or able to confirm via the state store, by a node.
		if node == nil {
			return structs.ErrTokenNotFound
		}

	}
	return nil
}
