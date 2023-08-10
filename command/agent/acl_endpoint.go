// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) ACLPoliciesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.ACLPolicyListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.ACLPolicyListResponse
	if err := s.agent.RPC("ACL.ListPolicies", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Policies == nil {
		out.Policies = make([]*structs.ACLPolicyListStub, 0)
	}
	return out.Policies, nil
}

func (s *HTTPServer) ACLPolicySpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name := strings.TrimPrefix(req.URL.Path, "/v1/acl/policy/")
	if len(name) == 0 {
		return nil, CodedError(400, "Missing Policy Name")
	}
	switch req.Method {
	case http.MethodGet:
		return s.aclPolicyQuery(resp, req, name)
	case http.MethodPut, http.MethodPost:
		return s.aclPolicyUpdate(resp, req, name)
	case http.MethodDelete:
		return s.aclPolicyDelete(resp, req, name)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) aclPolicyQuery(resp http.ResponseWriter, req *http.Request,
	policyName string) (interface{}, error) {
	args := structs.ACLPolicySpecificRequest{
		Name: policyName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleACLPolicyResponse
	if err := s.agent.RPC("ACL.GetPolicy", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Policy == nil {
		return nil, CodedError(404, "ACL policy not found")
	}
	return out.Policy, nil
}

func (s *HTTPServer) aclPolicyUpdate(resp http.ResponseWriter, req *http.Request,
	policyName string) (interface{}, error) {
	// Parse the policy
	var policy structs.ACLPolicy
	if err := decodeBody(req, &policy); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	// Ensure the policy name matches
	if policy.Name != policyName {
		return nil, CodedError(400, "ACL policy name does not match request path")
	}

	// Format the request
	args := structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{&policy},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("ACL.UpsertPolicies", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) aclPolicyDelete(resp http.ResponseWriter, req *http.Request,
	policyName string) (interface{}, error) {

	args := structs.ACLPolicyDeleteRequest{
		Names: []string{policyName},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("ACL.DeletePolicies", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) ACLTokensRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.ACLTokenListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.ACLTokenListResponse
	if err := s.agent.RPC("ACL.ListTokens", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Tokens == nil {
		out.Tokens = make([]*structs.ACLTokenListStub, 0)
	}
	return out.Tokens, nil
}

func (s *HTTPServer) ACLTokenBootstrap(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Ensure this is a PUT or POST
	if !(req.Method == "PUT" || req.Method == "POST") {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.ACLTokenBootstrapRequest

	if req.ContentLength != 0 {
		if err := decodeBody(req, &args); err != nil {
			return nil, CodedError(400, fmt.Sprintf("failed to decode request body: %s", err))
		}
	}

	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.ACLTokenUpsertResponse
	if err := s.agent.RPC("ACL.Bootstrap", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	if len(out.Tokens) > 0 {
		return out.Tokens[0], nil
	}
	return nil, nil
}

func (s *HTTPServer) ACLTokenSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := req.URL.Path

	switch path {
	case "/v1/acl/token":
		if !(req.Method == "PUT" || req.Method == "POST") {
			return nil, CodedError(405, ErrInvalidMethod)
		}
		return s.aclTokenUpdate(resp, req, "")
	case "/v1/acl/token/self":
		return s.aclTokenSelf(resp, req)
	}

	accessor := strings.TrimPrefix(path, "/v1/acl/token/")
	return s.aclTokenCrud(resp, req, accessor)
}

func (s *HTTPServer) aclTokenCrud(resp http.ResponseWriter, req *http.Request,
	tokenAccessor string) (interface{}, error) {
	if tokenAccessor == "" {
		return nil, CodedError(400, "Missing Token Accessor")
	}

	switch req.Method {
	case http.MethodGet:
		return s.aclTokenQuery(resp, req, tokenAccessor)
	case http.MethodPut, http.MethodPost:
		return s.aclTokenUpdate(resp, req, tokenAccessor)
	case http.MethodDelete:
		return s.aclTokenDelete(resp, req, tokenAccessor)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) aclTokenQuery(resp http.ResponseWriter, req *http.Request,
	tokenAccessor string) (interface{}, error) {
	args := structs.ACLTokenSpecificRequest{
		AccessorID: tokenAccessor,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleACLTokenResponse
	if err := s.agent.RPC("ACL.GetToken", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Token == nil {
		return nil, CodedError(404, "ACL token not found")
	}
	return out.Token, nil
}

func (s *HTTPServer) aclTokenSelf(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	args := structs.ResolveACLTokenRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	args.SecretID = args.AuthToken

	var out structs.ResolveACLTokenResponse
	if err := s.agent.RPC("ACL.ResolveToken", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Token == nil {
		return nil, CodedError(404, "ACL token not found")
	}
	return out.Token, nil
}

func (s *HTTPServer) aclTokenUpdate(resp http.ResponseWriter, req *http.Request,
	tokenAccessor string) (interface{}, error) {
	// Parse the token
	var token structs.ACLToken
	if err := decodeBody(req, &token); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	// Ensure the token accessor matches
	if tokenAccessor != "" && (token.AccessorID != tokenAccessor) {
		return nil, CodedError(400, "ACL token accessor does not match request path")
	}

	// Format the request
	args := structs.ACLTokenUpsertRequest{
		Tokens: []*structs.ACLToken{&token},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.ACLTokenUpsertResponse
	if err := s.agent.RPC("ACL.UpsertTokens", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	if len(out.Tokens) > 0 {
		return out.Tokens[0], nil
	}
	return nil, nil
}

func (s *HTTPServer) aclTokenDelete(resp http.ResponseWriter, req *http.Request,
	tokenAccessor string) (interface{}, error) {

	args := structs.ACLTokenDeleteRequest{
		AccessorIDs: []string{tokenAccessor},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("ACL.DeleteTokens", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) UpsertOneTimeToken(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Ensure this is a PUT or POST
	if !(req.Method == "PUT" || req.Method == "POST") {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// the request body is empty but we need to parse to get the auth token
	args := structs.OneTimeTokenUpsertRequest{}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.OneTimeTokenUpsertResponse
	if err := s.agent.RPC("ACL.UpsertOneTimeToken", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) ExchangeOneTimeToken(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Ensure this is a PUT or POST
	if !(req.Method == "PUT" || req.Method == "POST") {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.OneTimeTokenExchangeRequest
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.OneTimeTokenExchangeResponse
	if err := s.agent.RPC("ACL.ExchangeOneTimeToken", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

// ACLRoleListRequest performs a listing of ACL roles and is callable via the
// /v1/acl/roles HTTP API.
func (s *HTTPServer) ACLRoleListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// The endpoint only supports GET requests.
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Set up the request args and parse this to ensure the query options are
	// set.
	args := structs.ACLRolesListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	// Perform the RPC request.
	var reply structs.ACLRolesListResponse
	if err := s.agent.RPC(structs.ACLListRolesRPCMethod, &args, &reply); err != nil {
		return nil, err
	}

	setMeta(resp, &reply.QueryMeta)

	if reply.ACLRoles == nil {
		reply.ACLRoles = make([]*structs.ACLRoleListStub, 0)
	}
	return reply.ACLRoles, nil
}

// ACLRoleRequest creates a new ACL role and is callable via the
// /v1/acl/role HTTP API.
func (s *HTTPServer) ACLRoleRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// // The endpoint only supports PUT or POST requests.
	if !(req.Method == http.MethodPut || req.Method == http.MethodPost) {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Use the generic upsert function without setting an ID as this will be
	// handled by the Nomad leader.
	return s.aclRoleUpsertRequest(resp, req, "")
}

// ACLRoleSpecificRequest is callable via the /v1/acl/role/ HTTP API and
// handles read via both the role name and ID, updates, and deletions.
func (s *HTTPServer) ACLRoleSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// Grab the suffix of the request, so we can further understand it.
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/acl/role/")

	// Split the request suffix in order to identify whether this is a lookup
	// of a service, or whether this includes a service and service identifier.
	suffixParts := strings.Split(reqSuffix, "/")

	switch len(suffixParts) {
	case 1:
		// Ensure the role ID is not an empty string which is possible if the
		// caller requested "/v1/acl/role/"
		if suffixParts[0] == "" {
			return nil, CodedError(http.StatusBadRequest, "missing ACL role ID")
		}
		return s.aclRoleRequest(resp, req, suffixParts[0])
	case 2:
		// This endpoint only supports GET.
		if req.Method != http.MethodGet {
			return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
		}

		// Ensure that the path is correct, otherwise the call could use
		// "/v1/acl/role/foobar/role-name" and successfully pass through here.
		if suffixParts[0] != "name" {
			return nil, CodedError(http.StatusBadRequest, "invalid URI")
		}

		// Ensure the role name is not an empty string which is possible if the
		// caller requested "/v1/acl/role/name/"
		if suffixParts[1] == "" {
			return nil, CodedError(http.StatusBadRequest, "missing ACL role name")
		}

		return s.aclRoleGetByNameRequest(resp, req, suffixParts[1])

	default:
		return nil, CodedError(http.StatusBadRequest, "invalid URI")
	}
}

func (s *HTTPServer) aclRoleRequest(
	resp http.ResponseWriter, req *http.Request, roleID string) (interface{}, error) {

	// Identify the method which indicates which downstream function should be
	// called.
	switch req.Method {
	case http.MethodGet:
		return s.aclRoleGetByIDRequest(resp, req, roleID)
	case http.MethodDelete:
		return s.aclRoleDeleteRequest(resp, req, roleID)
	case http.MethodPost, http.MethodPut:
		return s.aclRoleUpsertRequest(resp, req, roleID)
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
}

func (s *HTTPServer) aclRoleGetByIDRequest(
	resp http.ResponseWriter, req *http.Request, roleID string) (interface{}, error) {

	args := structs.ACLRoleByIDRequest{
		RoleID: roleID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var reply structs.ACLRoleByIDResponse
	if err := s.agent.RPC(structs.ACLGetRoleByIDRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setMeta(resp, &reply.QueryMeta)

	if reply.ACLRole == nil {
		return nil, CodedError(http.StatusNotFound, "ACL role not found")
	}
	return reply.ACLRole, nil
}

func (s *HTTPServer) aclRoleDeleteRequest(
	resp http.ResponseWriter, req *http.Request, roleID string) (interface{}, error) {

	args := structs.ACLRolesDeleteByIDRequest{
		ACLRoleIDs: []string{roleID},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var reply structs.ACLRolesDeleteByIDResponse
	if err := s.agent.RPC(structs.ACLDeleteRolesByIDRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setIndex(resp, reply.Index)
	return nil, nil
}

// aclRoleUpsertRequest handles upserting an ACL to the Nomad servers. It can
// handle both new creations, and updates to existing roles.
func (s *HTTPServer) aclRoleUpsertRequest(
	resp http.ResponseWriter, req *http.Request, roleID string) (interface{}, error) {

	// Decode the ACL role.
	var aclRole structs.ACLRole
	if err := decodeBody(req, &aclRole); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	// Ensure the request path ID matches the ACL role ID that was decoded.
	// Only perform this check on updates as a generic error on creation might
	// be confusing to operators as there is no specific role request path.
	if roleID != "" && roleID != aclRole.ID {
		return nil, CodedError(http.StatusBadRequest, "ACL role ID does not match request path")
	}

	args := structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{&aclRole},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.ACLRolesUpsertResponse
	if err := s.agent.RPC(structs.ACLUpsertRolesRPCMethod, &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)

	if len(out.ACLRoles) > 0 {
		return out.ACLRoles[0], nil
	}
	return nil, nil
}

func (s *HTTPServer) aclRoleGetByNameRequest(
	resp http.ResponseWriter, req *http.Request, roleName string) (interface{}, error) {

	args := structs.ACLRoleByNameRequest{
		RoleName: roleName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var reply structs.ACLRoleByNameResponse
	if err := s.agent.RPC(structs.ACLGetRoleByNameRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setMeta(resp, &reply.QueryMeta)

	if reply.ACLRole == nil {
		return nil, CodedError(http.StatusNotFound, "ACL role not found")
	}
	return reply.ACLRole, nil
}

// ACLAuthMethodListRequest performs a listing of ACL auth-methods and is
// callable via the /v1/acl/auth-methods HTTP API.
func (s *HTTPServer) ACLAuthMethodListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// The endpoint only supports GET requests.
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Set up the request args and parse this to ensure the query options are
	// set.
	args := structs.ACLAuthMethodListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	// Perform the RPC request.
	var reply structs.ACLAuthMethodListResponse
	if err := s.agent.RPC(structs.ACLListAuthMethodsRPCMethod, &args, &reply); err != nil {
		return nil, err
	}

	setMeta(resp, &reply.QueryMeta)

	if reply.AuthMethods == nil {
		reply.AuthMethods = make([]*structs.ACLAuthMethodStub, 0)
	}
	return reply.AuthMethods, nil
}

// ACLAuthMethodRequest creates a new ACL auth-method and is callable via the
// /v1/acl/auth-method HTTP API.
func (s *HTTPServer) ACLAuthMethodRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// // The endpoint only supports PUT or POST requests.
	if !(req.Method == http.MethodPut || req.Method == http.MethodPost) {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Use the generic upsert function without setting an ID as this will be
	// handled by the Nomad leader.
	return s.aclAuthMethodUpsertRequest(resp, req, "")
}

// ACLAuthMethodSpecificRequest is callable via the /v1/acl/auth-method/ HTTP
// API and handles reads, updates, and deletions of named methods.
func (s *HTTPServer) ACLAuthMethodSpecificRequest(
	resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// Grab the suffix of the request, so we can further understand it.
	methodName := strings.TrimPrefix(req.URL.Path, "/v1/acl/auth-method/")

	// Ensure the auth-method name is not an empty string which is possible if
	// the caller requested "/v1/acl/role/auth-method/".
	if methodName == "" {
		return nil, CodedError(http.StatusBadRequest, "missing ACL auth-method name")
	}

	// Identify the method which indicates which downstream function should be
	// called.
	switch req.Method {
	case http.MethodGet:
		return s.aclAuthMethodGetRequest(resp, req, methodName)
	case http.MethodDelete:
		return s.aclAuthMethodDeleteRequest(resp, req, methodName)
	case http.MethodPost, http.MethodPut:
		return s.aclAuthMethodUpsertRequest(resp, req, methodName)
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
}

// aclAuthMethodGetRequest is callable via the /v1/acl/auth-method/ HTTP API
// and is used for reading the named auth-method from state.
func (s *HTTPServer) aclAuthMethodGetRequest(
	resp http.ResponseWriter, req *http.Request, methodName string) (interface{}, error) {

	args := structs.ACLAuthMethodGetRequest{
		MethodName: methodName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var reply structs.ACLAuthMethodGetResponse
	if err := s.agent.RPC(structs.ACLGetAuthMethodRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setMeta(resp, &reply.QueryMeta)

	if reply.AuthMethod == nil {
		return nil, CodedError(http.StatusNotFound, "ACL auth-method not found")
	}
	return reply.AuthMethod, nil
}

// aclAuthMethodDeleteRequest is callable via the /v1/acl/auth-method/ HTTP API
// and is responsible for deleting the named auth-method from state.
func (s *HTTPServer) aclAuthMethodDeleteRequest(
	resp http.ResponseWriter, req *http.Request, methodName string) (interface{}, error) {

	args := structs.ACLAuthMethodDeleteRequest{
		Names: []string{methodName},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var reply structs.ACLAuthMethodDeleteResponse
	if err := s.agent.RPC(structs.ACLDeleteAuthMethodsRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setIndex(resp, reply.Index)
	return nil, nil
}

// aclAuthMethodUpsertRequest handles upserting an ACL auth-method to the Nomad
// servers. It can handle both new creations, and updates to existing
// auth-methods.
func (s *HTTPServer) aclAuthMethodUpsertRequest(
	resp http.ResponseWriter, req *http.Request, methodName string) (interface{}, error) {

	// Decode the ACL auth-method.
	var aclAuthMethod structs.ACLAuthMethod
	if err := decodeBody(req, &aclAuthMethod); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	// Ensure the request path name matches the ACL auth-method name that was
	// decoded. Only perform this check on updates as a generic error on
	// creation might be confusing to operators as there is no specific
	// auth-method request path.
	if methodName != "" && methodName != aclAuthMethod.Name {
		return nil, CodedError(http.StatusBadRequest, "ACL auth-method name does not match request path")
	}

	args := structs.ACLAuthMethodUpsertRequest{
		AuthMethods: []*structs.ACLAuthMethod{&aclAuthMethod},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.ACLAuthMethodUpsertResponse
	if err := s.agent.RPC(structs.ACLUpsertAuthMethodsRPCMethod, &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)

	if len(out.AuthMethods) > 0 {
		return out.AuthMethods[0], nil
	}
	return nil, nil
}

// ACLBindingRuleListRequest performs a listing of ACL binding rules and is
// callable via the /v1/acl/binding-rules HTTP API.
func (s *HTTPServer) ACLBindingRuleListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// The endpoint only supports GET requests.
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Set up the request args and parse this to ensure the query options are
	// set.
	args := structs.ACLBindingRulesListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	// Perform the RPC request.
	var reply structs.ACLBindingRulesListResponse
	if err := s.agent.RPC(structs.ACLListBindingRulesRPCMethod, &args, &reply); err != nil {
		return nil, err
	}

	setMeta(resp, &reply.QueryMeta)

	if reply.ACLBindingRules == nil {
		reply.ACLBindingRules = make([]*structs.ACLBindingRuleListStub, 0)
	}
	return reply.ACLBindingRules, nil
}

// ACLBindingRuleRequest creates a new ACL binding rule and is callable via the
// /v1/acl/binding-rule HTTP API.
func (s *HTTPServer) ACLBindingRuleRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// // The endpoint only supports PUT or POST requests.
	if !(req.Method == http.MethodPut || req.Method == http.MethodPost) {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	// Use the generic upsert function without setting an ID as this will be
	// handled by the Nomad leader.
	return s.aclBindingRuleUpsertRequest(resp, req, "")
}

// ACLBindingRuleSpecificRequest is callable via the /v1/acl/binding-rule/ HTTP
// API and handles read via both the ID, updates, and deletions.
func (s *HTTPServer) ACLBindingRuleSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// Grab the suffix of the request, so we can further understand it.
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/acl/binding-rule/")

	// Ensure the binding rule ID is not an empty string which is possible if
	// the caller requested "/v1/acl/role/binding-rule/".
	if reqSuffix == "" {
		return nil, CodedError(http.StatusBadRequest, "missing ACL binding rule ID")
	}

	// Identify the HTTP method which indicates which downstream function
	// should be called.
	switch req.Method {
	case http.MethodGet:
		return s.aclBindingRuleGetRequest(resp, req, reqSuffix)
	case http.MethodDelete:
		return s.aclBindingRuleDeleteRequest(resp, req, reqSuffix)
	case http.MethodPost, http.MethodPut:
		return s.aclBindingRuleUpsertRequest(resp, req, reqSuffix)
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
}

func (s *HTTPServer) aclBindingRuleGetRequest(
	resp http.ResponseWriter, req *http.Request, ruleID string) (interface{}, error) {

	args := structs.ACLBindingRuleRequest{
		ACLBindingRuleID: ruleID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var reply structs.ACLBindingRuleResponse
	if err := s.agent.RPC(structs.ACLGetBindingRuleRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setMeta(resp, &reply.QueryMeta)

	if reply.ACLBindingRule == nil {
		return nil, CodedError(http.StatusNotFound, "ACL binding rule not found")
	}
	return reply.ACLBindingRule, nil
}

func (s *HTTPServer) aclBindingRuleDeleteRequest(
	resp http.ResponseWriter, req *http.Request, ruleID string) (interface{}, error) {

	args := structs.ACLBindingRulesDeleteRequest{
		ACLBindingRuleIDs: []string{ruleID},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var reply structs.ACLBindingRulesDeleteResponse
	if err := s.agent.RPC(structs.ACLDeleteBindingRulesRPCMethod, &args, &reply); err != nil {
		return nil, err
	}
	setIndex(resp, reply.Index)
	return nil, nil
}

// aclBindingRuleUpsertRequest handles upserting an ACL binding rule to the
// Nomad servers. It can handle both new creations, and updates to existing
// rules.
func (s *HTTPServer) aclBindingRuleUpsertRequest(
	resp http.ResponseWriter, req *http.Request, ruleID string) (interface{}, error) {

	// Decode the ACL binding rule.
	var aclBindingRule structs.ACLBindingRule
	if err := decodeBody(req, &aclBindingRule); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	// If the request path includes an ID, ensure the payload has an ID if it
	// has been left empty.
	if ruleID != "" && aclBindingRule.ID == "" {
		aclBindingRule.ID = ruleID
	}

	// Ensure the request path ID matches the ACL binding rule ID that was
	// decoded. Only perform this check on updates as a generic error on
	// creation might be confusing to operators as there is no specific binding
	// rule request path.
	if ruleID != "" && ruleID != aclBindingRule.ID {
		return nil, CodedError(http.StatusBadRequest, "ACL binding rule ID does not match request path")
	}

	args := structs.ACLBindingRulesUpsertRequest{
		ACLBindingRules: []*structs.ACLBindingRule{&aclBindingRule},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.ACLBindingRulesUpsertResponse
	if err := s.agent.RPC(structs.ACLUpsertBindingRulesRPCMethod, &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)

	if len(out.ACLBindingRules) > 0 {
		return out.ACLBindingRules[0], nil
	}
	return nil, nil
}

// ACLOIDCAuthURLRequest starts the OIDC login workflow.
func (s *HTTPServer) ACLOIDCAuthURLRequest(_ http.ResponseWriter, req *http.Request) (interface{}, error) {

	// The endpoint only supports PUT or POST requests.
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var args structs.ACLOIDCAuthURLRequest
	s.parseWriteRequest(req, &args.WriteRequest)

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	var out structs.ACLOIDCAuthURLResponse
	if err := s.agent.RPC(structs.ACLOIDCAuthURLRPCMethod, &args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ACLOIDCCompleteAuthRequest completes the OIDC login workflow.
func (s *HTTPServer) ACLOIDCCompleteAuthRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// The endpoint only supports PUT or POST requests.
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var args structs.ACLOIDCCompleteAuthRequest
	s.parseWriteRequest(req, &args.WriteRequest)

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	var out structs.ACLLoginResponse
	if err := s.agent.RPC(structs.ACLOIDCCompleteAuthRPCMethod, &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out.ACLToken, nil
}

// ACLLoginRequest performs a non-interactive authentication request
func (s *HTTPServer) ACLLoginRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	// The endpoint only supports PUT or POST requests.
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
	var args structs.ACLLoginRequest
	s.parseWriteRequest(req, &args.WriteRequest)
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}
	var out structs.ACLLoginResponse
	if err := s.agent.RPC(structs.ACLLoginRPCMethod, &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out.ACLToken, nil
}
