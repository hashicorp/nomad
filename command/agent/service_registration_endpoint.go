// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ServiceRegistrationListRequest performs a listing of service registrations
// using the structs.ServiceRegistrationListRPCMethod RPC endpoint and is
// callable via the /v1/services HTTP API.
func (s *HTTPServer) ServiceRegistrationListRequest(
	resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// The endpoint only supports GET requests.
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Set up the request args and parse this to ensure the query options are
	// set.
	args := structs.ServiceRegistrationListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	// Perform the RPC request.
	var reply structs.ServiceRegistrationListResponse
	if err := s.agent.RPC(structs.ServiceRegistrationListRPCMethod, &args, &reply); err != nil {
		return nil, err
	}

	setMeta(resp, &reply.QueryMeta)

	if reply.Services == nil {
		reply.Services = make([]*structs.ServiceRegistrationListStub, 0)
	}
	return reply.Services, nil
}

// ServiceRegistrationRequest is callable via the /v1/service/ HTTP API and
// handles service reads and individual service registration deletions.
func (s *HTTPServer) ServiceRegistrationRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// Grab the suffix of the request, so we can further understand it.
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/service/")

	// Split the request suffix in order to identify whether this is a lookup
	// of a service, or whether this includes a service and service identifier.
	suffixParts := strings.Split(reqSuffix, "/")

	switch len(suffixParts) {
	case 1:
		// This endpoint only supports GET.
		if req.Method != http.MethodGet {
			return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
		}

		// Ensure the service ID is not an empty string which is possible if
		// the caller requested "/v1/service/service-name/"
		if suffixParts[0] == "" {
			return nil, CodedError(http.StatusBadRequest, "missing service name")
		}

		return s.serviceGetRequest(resp, req, suffixParts[0])

	case 2:
		// This endpoint only supports DELETE.
		if req.Method != http.MethodDelete {
			return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
		}

		// Ensure the service ID is not an empty string which is possible if
		// the caller requested "/v1/service/service-name/"
		if suffixParts[1] == "" {
			return nil, CodedError(http.StatusBadRequest, "missing service id")
		}

		return s.serviceDeleteRequest(resp, req, suffixParts[1])

	default:
		return nil, CodedError(http.StatusBadRequest, "invalid URI")
	}
}

// serviceGetRequest performs a reading of service registrations by name using
// the structs.ServiceRegistrationGetServiceRPCMethod RPC endpoint.
func (s *HTTPServer) serviceGetRequest(
	resp http.ResponseWriter, req *http.Request, serviceName string) (interface{}, error) {

	args := structs.ServiceRegistrationByNameRequest{
		ServiceName: serviceName,
		Choose:      req.URL.Query().Get("choose"),
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var reply structs.ServiceRegistrationByNameResponse
	if err := s.agent.RPC(structs.ServiceRegistrationGetServiceRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setMeta(resp, &reply.QueryMeta)

	if reply.Services == nil {
		reply.Services = make([]*structs.ServiceRegistration, 0)
	}
	return reply.Services, nil
}

// serviceDeleteRequest performs a reading of service registrations by name using
// the structs.ServiceRegistrationDeleteByIDRPCMethod RPC endpoint.
func (s *HTTPServer) serviceDeleteRequest(
	resp http.ResponseWriter, req *http.Request, serviceID string) (interface{}, error) {

	args := structs.ServiceRegistrationDeleteByIDRequest{ID: serviceID}
	s.parseWriteRequest(req, &args.WriteRequest)

	var reply structs.ServiceRegistrationDeleteByIDResponse
	if err := s.agent.RPC(structs.ServiceRegistrationDeleteByIDRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setIndex(resp, reply.Index)
	return nil, nil
}
