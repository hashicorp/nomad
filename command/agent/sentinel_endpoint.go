// +build ent

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) SentinelPoliciesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.SentinelPolicyListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SentinelPolicyListResponse
	if err := s.agent.RPC("Sentinel.ListPolicies", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Policies == nil {
		out.Policies = make([]*structs.SentinelPolicyListStub, 0)
	}
	return out.Policies, nil
}

func (s *HTTPServer) SentinelPolicySpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name := strings.TrimPrefix(req.URL.Path, "/v1/sentinel/policy/")
	if len(name) == 0 {
		return nil, CodedError(400, "Missing Policy Name")
	}
	switch req.Method {
	case "GET":
		return s.sentinelPolicyQuery(resp, req, name)
	case "PUT", "POST":
		return s.sentinelPolicyUpdate(resp, req, name)
	case "DELETE":
		return s.sentinelPolicyDelete(resp, req, name)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) sentinelPolicyQuery(resp http.ResponseWriter, req *http.Request,
	policyName string) (interface{}, error) {
	args := structs.SentinelPolicySpecificRequest{
		Name: policyName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleSentinelPolicyResponse
	if err := s.agent.RPC("Sentinel.GetPolicy", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Policy == nil {
		return nil, CodedError(404, "Sentinel policy not found")
	}
	return out.Policy, nil
}

func (s *HTTPServer) sentinelPolicyUpdate(resp http.ResponseWriter, req *http.Request,
	policyName string) (interface{}, error) {
	// Parse the policy
	var policy structs.SentinelPolicy
	if err := decodeBody(req, &policy); err != nil {
		return nil, CodedError(500, err.Error())
	}

	// Ensure the policy name matches
	if policy.Name != policyName {
		return nil, CodedError(400, "Sentinel policy name does not match request path")
	}

	// Format the request
	args := structs.SentinelPolicyUpsertRequest{
		Policies: []*structs.SentinelPolicy{&policy},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Sentinel.UpsertPolicies", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) sentinelPolicyDelete(resp http.ResponseWriter, req *http.Request,
	policyName string) (interface{}, error) {

	args := structs.SentinelPolicyDeleteRequest{
		Names: []string{policyName},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Sentinel.DeletePolicies", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}
