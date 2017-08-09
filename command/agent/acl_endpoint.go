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
	s.parseRegion(req, &args.Region)

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
	s.parseRegion(req, &args.Region)

	var out structs.GenericResponse
	if err := s.agent.RPC("ACL.DeletePolicies", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}
