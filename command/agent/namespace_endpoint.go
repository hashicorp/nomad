// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) NamespacesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.NamespaceListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.NamespaceListResponse
	if err := s.agent.RPC("Namespace.ListNamespaces", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Namespaces == nil {
		out.Namespaces = make([]*structs.Namespace, 0)
	}
	return out.Namespaces, nil
}

func (s *HTTPServer) NamespaceSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name := strings.TrimPrefix(req.URL.Path, "/v1/namespace/")
	if len(name) == 0 {
		return nil, CodedError(400, "Missing Namespace Name")
	}
	switch req.Method {
	case http.MethodGet:
		return s.namespaceQuery(resp, req, name)
	case http.MethodPut, http.MethodPost:
		return s.namespaceUpdate(resp, req, name)
	case http.MethodDelete:
		return s.namespaceDelete(resp, req, name)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) NamespaceCreateRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPut && req.Method != http.MethodPost {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	return s.namespaceUpdate(resp, req, "")
}

func (s *HTTPServer) namespaceQuery(resp http.ResponseWriter, req *http.Request,
	namespaceName string) (interface{}, error) {
	args := structs.NamespaceSpecificRequest{
		Name: namespaceName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleNamespaceResponse
	if err := s.agent.RPC("Namespace.GetNamespace", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Namespace == nil {
		return nil, CodedError(404, "Namespace not found")
	}
	return out.Namespace, nil
}

func (s *HTTPServer) namespaceUpdate(resp http.ResponseWriter, req *http.Request,
	namespaceName string) (interface{}, error) {
	// Parse the namespace
	var namespace structs.Namespace
	if err := decodeBody(req, &namespace); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	// Ensure the namespace name matches
	if namespaceName != "" && namespace.Name != namespaceName {
		return nil, CodedError(400, "Namespace name does not match request path")
	}

	// Format the request
	args := structs.NamespaceUpsertRequest{
		Namespaces: []*structs.Namespace{&namespace},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Namespace.UpsertNamespaces", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) namespaceDelete(resp http.ResponseWriter, req *http.Request,
	namespaceName string) (interface{}, error) {

	args := structs.NamespaceDeleteRequest{
		Namespaces: []string{namespaceName},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Namespace.DeleteNamespaces", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}
