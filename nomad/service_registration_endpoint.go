// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ServiceRegistration encapsulates the service registrations RPC endpoint
// which is callable via the ServiceRegistration RPCs and externally via the
// "/v1/service{s}" HTTP API.
type ServiceRegistration struct {
	srv *Server
	ctx *RPCContext
}

func NewServiceRegistrationEndpoint(srv *Server, ctx *RPCContext) *ServiceRegistration {
	return &ServiceRegistration{srv: srv, ctx: ctx}
}

// Upsert creates or updates service registrations held within Nomad. This RPC
// is only callable by Nomad nodes.
func (s *ServiceRegistration) Upsert(
	args *structs.ServiceRegistrationUpsertRequest,
	reply *structs.ServiceRegistrationUpsertResponse) error {

	authErr := s.srv.Authenticate(s.ctx, args)

	// Ensure the connection was initiated by a client if TLS is used.
	if err := validateTLSCertificateLevel(s.srv, s.ctx, tlsCertificateLevelClient); err != nil {
		return err
	}
	if done, err := s.srv.forward(structs.ServiceRegistrationUpsertRPCMethod, args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("service_registration", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "upsert"}, time.Now())

	// Nomad service registrations can only be used once all servers, in the
	// local region, have been upgraded to 1.3.0 or greater.
	if !ServersMeetMinimumVersion(s.srv.Members(), s.srv.Region(), minNomadServiceRegistrationVersion, false) {
		return fmt.Errorf("all servers should be running version %v or later to use the Nomad service provider",
			minNomadServiceRegistrationVersion)
	}

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
	_, index, err := s.srv.raftApply(structs.ServiceRegistrationUpsertRequestType, args)
	if err != nil {
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

	authErr := s.srv.Authenticate(s.ctx, args)
	if done, err := s.srv.forward(structs.ServiceRegistrationDeleteByIDRPCMethod, args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("service_registration", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "delete_id"}, time.Now())

	// Nomad service registrations can only be used once all servers, in the
	// local region, have been upgraded to 1.3.0 or greater.
	if !ServersMeetMinimumVersion(s.srv.Members(), s.srv.Region(), minNomadServiceRegistrationVersion, false) {
		return fmt.Errorf("all servers should be running version %v or later to use the Nomad service provider",
			minNomadServiceRegistrationVersion)
	}

	// Perform the ACL token resolution.
	aclObj, err := s.srv.ResolveACL(args)

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
	_, index, err := s.srv.raftApply(structs.ServiceRegistrationDeleteByIDRequestType, args)
	if err != nil {
		return err
	}

	// Update the index. There is no need to floor this as we are writing to
	// state and therefore will get a non-zero index response.
	reply.Index = index
	return nil
}

// serviceTagSet maps from a service name to a union of tags associated with that service.
type serviceTagSet map[string]*set.Set[string]

func (s serviceTagSet) add(service string, tags []string) {
	if _, exists := s[service]; !exists {
		s[service] = set.From[string](tags)
	} else {
		s[service].InsertAll(tags)
	}
}

// namespaceServiceTagSet maps from a namespace to a serviceTagSet
type namespaceServiceTagSet map[string]serviceTagSet

func (s namespaceServiceTagSet) add(namespace, service string, tags []string) {
	if _, exists := s[namespace]; !exists {
		s[namespace] = make(serviceTagSet)
	}
	s[namespace].add(service, tags)
}

// List is used to list service registration held within state. It supports
// single and wildcard namespace listings.
func (s *ServiceRegistration) List(
	args *structs.ServiceRegistrationListRequest,
	reply *structs.ServiceRegistrationListResponse) error {

	authErr := s.srv.Authenticate(s.ctx, args)
	if done, err := s.srv.forward(structs.ServiceRegistrationListRPCMethod, args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("service_registration", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "list"}, time.Now())

	// If the caller has requested to list services across all namespaces, use
	// the custom function to perform this.
	if args.RequestNamespace() == structs.AllNamespacesSentinel {
		return s.listAllServiceRegistrations(args, reply)
	}

	aclObj, err := s.srv.ResolveClientOrACL(args)
	if err != nil {
		return structs.ErrPermissionDenied
	}
	if args.GetIdentity().Claims == nil &&
		!aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
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

			// Accumulate the set of tags associated with a particular service name.
			tagSet := make(serviceTagSet)

			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				serviceReg := raw.(*structs.ServiceRegistration)
				tagSet.add(serviceReg.ServiceName, serviceReg.Tags)
			}

			// Set the output result with the accumulated set of tags for each service.
			var serviceList []*structs.ServiceRegistrationStub
			for service, tags := range tagSet {
				serviceList = append(serviceList, &structs.ServiceRegistrationStub{
					ServiceName: service,
					Tags:        tags.List(),
				})
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

	// Perform ACL resolution. The request already goes through forwarding
	// and metrics setup before being called.
	aclObj, err := s.srv.ResolveACL(args)
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

			// Accumulate the union of tags per service in each namespace.
			nsSvcTagSet := make(namespaceServiceTagSet)

			// Iterate all service registrations.
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				reg := raw.(*structs.ServiceRegistration)

				// Check whether the service registration is within a namespace
				// the caller is permitted to view. nil allowedNSes means the
				// caller can view all namespaces.
				if allowedNSes != nil && !allowedNSes[reg.Namespace] {
					continue
				}

				// Accumulate the set of tags associated with a particular service name in a particular namespace
				nsSvcTagSet.add(reg.Namespace, reg.ServiceName, reg.Tags)
			}

			// Create the service stubs, one per namespace, containing each service
			// in that namespace, and append that to the final tally of registrations.
			var registrations []*structs.ServiceRegistrationListStub
			for namespace, tagSet := range nsSvcTagSet {
				var stubs []*structs.ServiceRegistrationStub
				for service, tags := range tagSet {
					stubs = append(stubs, &structs.ServiceRegistrationStub{
						ServiceName: service,
						Tags:        tags.List(),
					})
				}
				registrations = append(registrations, &structs.ServiceRegistrationListStub{
					Namespace: namespace,
					Services:  stubs,
				})
			}

			// Set the output on the reply object.
			reply.Services = registrations

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

	authErr := s.srv.Authenticate(s.ctx, args)
	if done, err := s.srv.forward(structs.ServiceRegistrationGetServiceRPCMethod, args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("service_registration", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "service_registration", "get_service"}, time.Now())

	aclObj, err := s.srv.ResolveClientOrACL(args)
	if err != nil {
		return structs.ErrPermissionDenied
	}
	if args.GetIdentity().Claims == nil &&
		!aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
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

			// Select which subset and the order of services to return if using ?choose
			if args.Choose != "" {
				chosen, chooseErr := s.choose(services, args.Choose)
				if chooseErr != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to choose services: %v", chooseErr)
				}
				services = chosen
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

// choose uses rendezvous hashing to make a stable selection of a subset of services
// to return.
//
// parameter must in the form "<number>|<key>", where number is the number of services
// to select, and key is incorporated in the hashing function with each service -
// creating a unique yet consistent priority distribution pertaining to the requester.
// In practice (i.e. via consul-template), the key is the AllocID generating a request
// for upstream services.
//
// https://en.wikipedia.org/wiki/Rendezvous_hashing
// w := priority (i.e. hash value)
// h := hash function
// O := object - (i.e. requesting service - using key (allocID) as a proxy)
// S := site (i.e. destination service)
func (*ServiceRegistration) choose(services []*structs.ServiceRegistration, parameter string) ([]*structs.ServiceRegistration, error) {
	// extract the number of services
	tokens := strings.SplitN(parameter, "|", 2)
	if len(tokens) != 2 {
		return nil, structs.ErrMalformedChooseParameter
	}
	n, err := strconv.Atoi(tokens[0])
	if err != nil {
		return nil, structs.ErrMalformedChooseParameter
	}

	// extract the hash key
	key := tokens[1]
	if key == "" {
		return nil, structs.ErrMalformedChooseParameter
	}

	// if there are fewer services than requested, go with the number of services
	if l := len(services); l < n {
		n = l
	}

	type pair struct {
		hash    string
		service *structs.ServiceRegistration
	}

	// associate hash for each service
	priorities := make([]*pair, len(services))
	for i, service := range services {
		priorities[i] = &pair{
			hash:    service.HashWith(key),
			service: service,
		}
	}

	// sort by the hash; creating random distribution of priority
	sort.SliceStable(priorities, func(i, j int) bool {
		return priorities[i].hash < priorities[j].hash
	})

	// choose top n services
	chosen := make([]*structs.ServiceRegistration, n)
	for i := 0; i < n; i++ {
		chosen[i] = priorities[i].service
	}

	return chosen, nil
}
