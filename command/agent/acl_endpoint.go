package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) ACLPoliciesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
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
	case "GET":
		return s.aclPolicyQuery(resp, req, name)
	case "PUT", "POST":
		return s.aclPolicyUpdate(resp, req, name)
	case "DELETE":
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
		return nil, CodedError(500, err.Error())
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
	if req.Method != "GET" {
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

	// Format the request
	args := structs.ACLTokenBootstrapRequest{}
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
	accessor := strings.TrimPrefix(req.URL.Path, "/v1/acl/token")

	// If there is no accessor, this must be a create
	if len(accessor) == 0 {
		if !(req.Method == "PUT" || req.Method == "POST") {
			return nil, CodedError(405, ErrInvalidMethod)
		}
		return s.aclTokenUpdate(resp, req, "")
	}

	// Check if no accessor is given past the slash
	accessor = accessor[1:]
	if accessor == "" {
		return nil, CodedError(400, "Missing Token Accessor")
	}

	switch req.Method {
	case "GET":
		return s.aclTokenQuery(resp, req, accessor)
	case "PUT", "POST":
		return s.aclTokenUpdate(resp, req, accessor)
	case "DELETE":
		return s.aclTokenDelete(resp, req, accessor)
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

func (s *HTTPServer) aclTokenUpdate(resp http.ResponseWriter, req *http.Request,
	tokenAccessor string) (interface{}, error) {
	// Parse the token
	var token structs.ACLToken
	if err := decodeBody(req, &token); err != nil {
		return nil, CodedError(500, err.Error())
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
